package ports

import (
	"reflect"
	"testing"
)

func TestParseProcNetTCPListenLine(t *testing.T) {
	line := "   0: 0100007F:0BB8 00000000:0000 0A 00000000:00000000 00:00000000 00000000 1000 0 12345 1 0000000000000000 100 0 0 10 0"

	entry, ok := parseProcNetTCPLine(line)
	if !ok {
		t.Fatal("expected line to parse")
	}
	if entry.Port != 3000 || entry.Inode != "12345" {
		t.Fatalf("entry = %#v, want port 3000 inode 12345", entry)
	}
}

func TestParseProcNetTCPLineSkipsNonListeningSockets(t *testing.T) {
	line := "   0: 0100007F:0BB8 00000000:0000 01 00000000:00000000 00:00000000 00000000 1000 0 12345 1 0000000000000000 100 0 0 10 0"

	if _, ok := parseProcNetTCPLine(line); ok {
		t.Fatal("expected non-listening socket to be skipped")
	}
}

func TestMapProcessesToProjects(t *testing.T) {
	portsByInode := map[string]int{
		"111": 3000,
		"222": 8787,
		"333": 9230,
		"444": 55595,
		"555": 9000,
	}
	processes := []processSockets{
		{Cwd: "/home/me/dev/app", Inodes: []string{"111", "333", "444"}},
		{Cwd: "/home/me/dev/app/api", Inodes: []string{"222"}},
		{Cwd: "/home/me/dev/other", Inodes: []string{"555"}},
	}

	got := mapProcessesToProjects([]string{"/home/me/dev/app"}, portsByInode, processes)
	want := map[string][]int{"/home/me/dev/app": {3000, 8787}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("mapProcessesToProjects() = %#v, want %#v", got, want)
	}
}

func TestParseLsofListenFields(t *testing.T) {
	text := "p100\nn127.0.0.1:3000\nn127.0.0.1:9229\np200\nn*:5173\n"

	got := parseLsofListenFields(text)
	want := map[int][]int{100: {3000, 9229}, 200: {5173}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("parseLsofListenFields() = %#v, want %#v", got, want)
	}
}

func TestParseLsofCwdFields(t *testing.T) {
	text := "p100\nn/Users/me/dev/app\np200\nn/Users/me/dev/other\n"

	got := parseLsofCwdFields(text)
	want := map[int]string{100: "/Users/me/dev/app", 200: "/Users/me/dev/other"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("parseLsofCwdFields() = %#v, want %#v", got, want)
	}
}

func TestMapProcessPortsToProjects(t *testing.T) {
	portsByPID := map[int][]int{
		100: {3000, 9229, 55595},
		200: {8787},
		300: {9000},
	}
	cwdByPID := map[int]string{
		100: "/Users/me/dev/app",
		200: "/Users/me/dev/app/api",
		300: "/Users/me/dev/other",
	}

	got := mapProcessPortsToProjects([]string{"/Users/me/dev/app"}, portsByPID, cwdByPID)
	want := map[string][]int{"/Users/me/dev/app": {3000, 8787}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("mapProcessPortsToProjects() = %#v, want %#v", got, want)
	}
}

func TestParsePowerShellPortProcesses(t *testing.T) {
	text := "\"OwningProcess\",\"LocalPort\",\"CommandLine\",\"ExecutablePath\"\n" +
		"\"100\",\"3000\",\"node C:\\Users\\me\\dev\\app\\node_modules\\.bin\\vite\",\"C:\\Program Files\\nodejs\\node.exe\"\n" +
		"\"100\",\"9229\",\"node C:\\Users\\me\\dev\\app\\node_modules\\.bin\\vite\",\"C:\\Program Files\\nodejs\\node.exe\"\n" +
		"\"200\",\"5173\",\"node C:\\Users\\me\\dev\\other\\server.js\",\"C:\\Program Files\\nodejs\\node.exe\"\n"

	got := parsePowerShellPortProcesses(text)
	want := []processPorts{
		{PID: 100, Ports: []int{3000}, PathHint: "node C:\\Users\\me\\dev\\app\\node_modules\\.bin\\vite C:\\Program Files\\nodejs\\node.exe"},
		{PID: 200, Ports: []int{5173}, PathHint: "node C:\\Users\\me\\dev\\other\\server.js C:\\Program Files\\nodejs\\node.exe"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("parsePowerShellPortProcesses() = %#v, want %#v", got, want)
	}
}

func TestMapPortHintsToProjectsUsesCommandLinePath(t *testing.T) {
	processes := []processPorts{
		{PID: 100, Ports: []int{3000, 9229}, PathHint: "node C:\\Users\\me\\dev\\app\\node_modules\\.bin\\vite"},
		{PID: 200, Ports: []int{5173}, PathHint: "node C:\\Users\\me\\dev\\other\\server.js"},
	}

	got := mapPortHintsToProjects([]string{"C:/Users/me/dev/app"}, processes)
	want := map[string][]int{"C:/Users/me/dev/app": {3000}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("mapPortHintsToProjects() = %#v, want %#v", got, want)
	}
}

func TestMapPortHintsToProjectsRejectsSiblingPathPrefix(t *testing.T) {
	processes := []processPorts{
		{PID: 100, Ports: []int{3000}, PathHint: "node /home/me/dev/app-old/server.js"},
		{PID: 200, Ports: []int{5173}, PathHint: "node /home/me/dev/application/server.js"},
	}

	got := mapPortHintsToProjects([]string{"/home/me/dev/app"}, processes)
	if len(got) != 0 {
		t.Fatalf("mapPortHintsToProjects() = %#v, want no matches", got)
	}
}

func TestMapPortHintsToProjectsRejectsWindowsSiblingPathPrefix(t *testing.T) {
	processes := []processPorts{
		{PID: 100, Ports: []int{3000}, PathHint: "node C:\\Users\\me\\dev\\app-old\\server.js"},
		{PID: 200, Ports: []int{5173}, PathHint: "node C:\\Users\\me\\dev\\application\\server.js"},
	}

	got := mapPortHintsToProjects([]string{"C:/Users/me/dev/app"}, processes)
	if len(got) != 0 {
		t.Fatalf("mapPortHintsToProjects() = %#v, want no matches", got)
	}
}
