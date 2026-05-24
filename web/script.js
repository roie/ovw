const installCommands = {
  homebrew: {
    label: "Recommended for macOS and Linux with Homebrew",
    command: "brew tap roie/tap\nbrew install ovw",
  },
  curl: {
    label: "Recommended for macOS and Linux without Homebrew",
    command: "curl -fsSL https://raw.githubusercontent.com/roie/ovw/main/install.sh | sh",
  },
  powershell: {
    label: "Recommended for Windows",
    command: 'powershell -c "irm https://raw.githubusercontent.com/roie/ovw/main/install.ps1 | iex"',
  },
  manual: {
    label: "Download a prebuilt binary from GitHub Releases",
    command: "https://github.com/roie/ovw/releases",
  },
};

const tabs = [...document.querySelectorAll("[data-install-tab]")];
const command = document.querySelector("[data-command]");
const detected = document.querySelector("#install-detected");
const copyButton = document.querySelector("[data-copy]");
let copyResetTimer;

function detectInstallMethod() {
  const platform = navigator.userAgentData?.platform || navigator.platform || "";
  const userAgent = navigator.userAgent || "";
  const value = `${platform} ${userAgent}`.toLowerCase();
  const isMobile = navigator.userAgentData?.mobile || navigator.maxTouchPoints > 1;

  if (value.includes("android") || value.includes("iphone") || value.includes("ipad") || value.includes("ios") || (value.includes("mac") && isMobile)) {
    return "manual";
  }
  if (value.includes("win")) {
    return "powershell";
  }
  if (value.includes("linux")) {
    return "curl";
  }
  if (value.includes("mac")) {
    return "homebrew";
  }
  return "manual";
}

function setInstallMethod(method) {
  const selected = installCommands[method] ? method : "manual";
  tabs.forEach((tab) => {
    const active = tab.dataset.installTab === selected;
    tab.classList.toggle("is-active", active);
    tab.setAttribute("aria-selected", String(active));
  });
  command.textContent = installCommands[selected].command;
  detected.textContent = installCommands[selected].label;
  copyButton.textContent = "copy";
}

function selectCommandText() {
  const selection = window.getSelection?.();
  if (!selection || !document.createRange) {
    return false;
  }
  const range = document.createRange();
  range.selectNodeContents(command);
  selection.removeAllRanges();
  selection.addRange(range);
  return true;
}

tabs.forEach((tab) => {
  tab.addEventListener("click", () => setInstallMethod(tab.dataset.installTab));
});

copyButton.addEventListener("click", async () => {
  clearTimeout(copyResetTimer);
  try {
    await navigator.clipboard.writeText(command.textContent);
    copyButton.textContent = "copied";
  } catch {
    copyButton.textContent = selectCommandText() ? "press ctrl+c" : "copy failed";
  }
  copyResetTimer = setTimeout(() => {
    copyButton.textContent = "copy";
  }, 1600);
});

setInstallMethod(detectInstallMethod());
