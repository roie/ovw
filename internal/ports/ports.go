package ports

import (
	"bufio"
	"encoding/csv"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
)

type procNetEntry struct {
	Port  int
	Inode string
}

type processSockets struct {
	Cwd    string
	Inodes []string
}

type processPorts struct {
	PID      int
	Ports    []int
	PathHint string
}

func Detect(projectPaths []string) map[string][]int {
	switch runtime.GOOS {
	case "linux":
		return detectLinux(projectPaths)
	case "darwin":
		return detectDarwin(projectPaths)
	case "windows":
		return detectWindows(projectPaths)
	default:
		return map[string][]int{}
	}
}

func detectLinux(projectPaths []string) map[string][]int {
	portsByInode := listeningPortsByInode()
	if len(portsByInode) == 0 {
		return map[string][]int{}
	}
	return mapProcessesToProjects(projectPaths, portsByInode, processSocketList("/proc"))
}

func detectDarwin(projectPaths []string) map[string][]int {
	out, err := exec.Command("lsof", "-nP", "-iTCP", "-sTCP:LISTEN", "-F", "pn").Output()
	if err != nil || len(out) == 0 {
		return map[string][]int{}
	}
	portsByPID := parseLsofListenFields(string(out))
	if len(portsByPID) == 0 {
		return map[string][]int{}
	}

	cwdByPID := map[int]string{}
	for pid := range portsByPID {
		out, err := exec.Command("lsof", "-a", "-p", strconv.Itoa(pid), "-d", "cwd", "-F", "pn").Output()
		if err != nil || len(out) == 0 {
			continue
		}
		for parsedPID, cwd := range parseLsofCwdFields(string(out)) {
			cwdByPID[parsedPID] = cwd
		}
	}
	return mapProcessPortsToProjects(projectPaths, portsByPID, cwdByPID)
}

func detectWindows(projectPaths []string) map[string][]int {
	script := `Get-NetTCPConnection -State Listen | ForEach-Object {
  $p = Get-CimInstance Win32_Process -Filter "ProcessId=$($_.OwningProcess)" -ErrorAction SilentlyContinue
  [PSCustomObject]@{
    OwningProcess = $_.OwningProcess
    LocalPort = $_.LocalPort
    CommandLine = if ($p) { $p.CommandLine } else { "" }
    ExecutablePath = if ($p) { $p.ExecutablePath } else { "" }
  }
} | ConvertTo-Csv -NoTypeInformation`
	out, err := exec.Command("powershell", "-NoProfile", "-Command", script).Output()
	if err != nil || len(out) == 0 {
		return map[string][]int{}
	}
	return mapPortHintsToProjects(projectPaths, parsePowerShellPortProcesses(string(out)))
}

func listeningPortsByInode() map[string]int {
	portsByInode := map[string]int{}
	for _, path := range []string{"/proc/net/tcp", "/proc/net/tcp6"} {
		for _, entry := range parseProcNetTCP(path) {
			portsByInode[entry.Inode] = entry.Port
		}
	}
	return portsByInode
}

func parseProcNetTCP(path string) []procNetEntry {
	file, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer file.Close()

	entries := []procNetEntry{}
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		if entry, ok := parseProcNetTCPLine(scanner.Text()); ok {
			entries = append(entries, entry)
		}
	}
	return entries
}

func parseProcNetTCPLine(line string) (procNetEntry, bool) {
	fields := strings.Fields(line)
	if len(fields) < 10 || fields[0] == "sl" || fields[3] != "0A" {
		return procNetEntry{}, false
	}
	local := strings.Split(fields[1], ":")
	if len(local) != 2 {
		return procNetEntry{}, false
	}
	port64, err := strconv.ParseInt(local[1], 16, 32)
	if err != nil || port64 <= 0 {
		return procNetEntry{}, false
	}
	return procNetEntry{Port: int(port64), Inode: fields[9]}, true
}

func processSocketList(procRoot string) []processSockets {
	entries, err := os.ReadDir(procRoot)
	if err != nil {
		return nil
	}
	processes := []processSockets{}
	for _, entry := range entries {
		if !entry.IsDir() || !isPID(entry.Name()) {
			continue
		}
		pidRoot := filepath.Join(procRoot, entry.Name())
		cwd, err := os.Readlink(filepath.Join(pidRoot, "cwd"))
		if err != nil || cwd == "" {
			continue
		}
		inodes := socketInodes(filepath.Join(pidRoot, "fd"))
		if len(inodes) == 0 {
			continue
		}
		processes = append(processes, processSockets{Cwd: cwd, Inodes: inodes})
	}
	return processes
}

func isPID(value string) bool {
	if value == "" {
		return false
	}
	for _, r := range value {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

func socketInodes(fdDir string) []string {
	entries, err := os.ReadDir(fdDir)
	if err != nil {
		return nil
	}
	inodes := []string{}
	for _, entry := range entries {
		target, err := os.Readlink(filepath.Join(fdDir, entry.Name()))
		if err != nil || !strings.HasPrefix(target, "socket:[") || !strings.HasSuffix(target, "]") {
			continue
		}
		inodes = append(inodes, strings.TrimSuffix(strings.TrimPrefix(target, "socket:["), "]"))
	}
	return inodes
}

func mapProcessesToProjects(projectPaths []string, portsByInode map[string]int, processes []processSockets) map[string][]int {
	result := map[string][]int{}
	for _, process := range processes {
		projectPath, ok := containingProject(process.Cwd, projectPaths)
		if !ok {
			continue
		}
		for _, inode := range process.Inodes {
			port, ok := portsByInode[inode]
			if !ok || !isDisplayPort(port) {
				continue
			}
			result[projectPath] = appendUniquePort(result[projectPath], port)
		}
	}
	for path := range result {
		sort.Ints(result[path])
	}
	return result
}

func parseLsofListenFields(text string) map[int][]int {
	portsByPID := map[int][]int{}
	pid := 0
	scanner := bufio.NewScanner(strings.NewReader(text))
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}
		switch line[0] {
		case 'p':
			parsed, err := strconv.Atoi(strings.TrimSpace(line[1:]))
			if err == nil {
				pid = parsed
			}
		case 'n':
			if pid == 0 {
				continue
			}
			port, ok := portFromEndpoint(line[1:])
			if ok {
				portsByPID[pid] = appendUniquePort(portsByPID[pid], port)
			}
		}
	}
	return portsByPID
}

func parseLsofCwdFields(text string) map[int]string {
	cwdByPID := map[int]string{}
	pid := 0
	scanner := bufio.NewScanner(strings.NewReader(text))
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}
		switch line[0] {
		case 'p':
			parsed, err := strconv.Atoi(strings.TrimSpace(line[1:]))
			if err == nil {
				pid = parsed
			}
		case 'n':
			if pid != 0 && strings.TrimSpace(line[1:]) != "" {
				cwdByPID[pid] = strings.TrimSpace(line[1:])
			}
		}
	}
	return cwdByPID
}

func portFromEndpoint(endpoint string) (int, bool) {
	index := strings.LastIndex(endpoint, ":")
	if index < 0 || index == len(endpoint)-1 {
		return 0, false
	}
	port, err := strconv.Atoi(endpoint[index+1:])
	if err != nil || port <= 0 {
		return 0, false
	}
	return port, true
}

func mapProcessPortsToProjects(projectPaths []string, portsByPID map[int][]int, cwdByPID map[int]string) map[string][]int {
	processes := make([]processPorts, 0, len(portsByPID))
	for pid, ports := range portsByPID {
		processes = append(processes, processPorts{PID: pid, Ports: ports, PathHint: cwdByPID[pid]})
	}
	return mapPortHintsToProjects(projectPaths, processes)
}

func parsePowerShellPortProcesses(text string) []processPorts {
	reader := csv.NewReader(strings.NewReader(text))
	reader.FieldsPerRecord = -1
	records, err := reader.ReadAll()
	if err != nil || len(records) < 2 {
		return nil
	}
	header := csvHeader(records[0])
	byPID := map[int]processPorts{}
	for _, record := range records[1:] {
		pid, err := strconv.Atoi(csvField(record, header, "OwningProcess"))
		if err != nil || pid <= 0 {
			continue
		}
		port, err := strconv.Atoi(csvField(record, header, "LocalPort"))
		if err != nil || !isDisplayPort(port) {
			continue
		}
		process := byPID[pid]
		process.PID = pid
		process.Ports = appendUniquePort(process.Ports, port)
		hint := strings.TrimSpace(strings.Join([]string{
			csvField(record, header, "CommandLine"),
			csvField(record, header, "ExecutablePath"),
		}, " "))
		if hint != "" {
			process.PathHint = hint
		}
		byPID[pid] = process
	}
	processes := make([]processPorts, 0, len(byPID))
	for _, process := range byPID {
		sort.Ints(process.Ports)
		processes = append(processes, process)
	}
	sort.Slice(processes, func(i, j int) bool {
		return processes[i].PID < processes[j].PID
	})
	return processes
}

func csvHeader(record []string) map[string]int {
	header := map[string]int{}
	for index, value := range record {
		header[value] = index
	}
	return header
}

func csvField(record []string, header map[string]int, name string) string {
	index, ok := header[name]
	if !ok || index < 0 || index >= len(record) {
		return ""
	}
	return record[index]
}

func mapPortHintsToProjects(projectPaths []string, processes []processPorts) map[string][]int {
	result := map[string][]int{}
	for _, process := range processes {
		projectPath, ok := containingProject(process.PathHint, projectPaths)
		if !ok {
			continue
		}
		for _, port := range process.Ports {
			if !isDisplayPort(port) {
				continue
			}
			result[projectPath] = appendUniquePort(result[projectPath], port)
		}
	}
	for path := range result {
		sort.Ints(result[path])
	}
	return result
}

func isDisplayPort(port int) bool {
	if port >= 49152 {
		return false
	}
	if port >= 9229 && port <= 9239 {
		return false
	}
	return true
}

func containingProject(cwd string, projectPaths []string) (string, bool) {
	best := ""
	for _, projectPath := range projectPaths {
		if matchesProjectPath(projectPath, cwd) && len(projectPath) > len(best) {
			best = projectPath
		}
	}
	return best, best != ""
}

func matchesProjectPath(projectPath, value string) bool {
	if pathContains(projectPath, value) {
		return true
	}
	projectPath = normalizePathText(projectPath)
	value = normalizePathText(value)
	return strings.Contains(value, projectPath)
}

func normalizePathText(value string) string {
	value = strings.ReplaceAll(value, "\\", "/")
	if runtime.GOOS == "windows" {
		value = strings.ToLower(value)
	}
	return value
}

func pathContains(parent, child string) bool {
	rel, err := filepath.Rel(parent, child)
	if err != nil {
		return false
	}
	return rel == "." || (rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)))
}

func appendUniquePort(ports []int, port int) []int {
	for _, existing := range ports {
		if existing == port {
			return ports
		}
	}
	return append(ports, port)
}
