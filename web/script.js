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

function detectInstallMethod() {
  const platform = navigator.userAgentData?.platform || navigator.platform || "";
  const userAgent = navigator.userAgent || "";
  const value = `${platform} ${userAgent}`.toLowerCase();

  if (value.includes("win")) {
    return "powershell";
  }
  if (value.includes("linux")) {
    return "curl";
  }
  return "homebrew";
}

function setInstallMethod(method) {
  const selected = installCommands[method] ? method : "homebrew";
  tabs.forEach((tab) => {
    const active = tab.dataset.installTab === selected;
    tab.classList.toggle("is-active", active);
    tab.setAttribute("aria-selected", String(active));
  });
  command.textContent = installCommands[selected].command;
  detected.textContent = installCommands[selected].label;
  copyButton.textContent = "copy";
}

tabs.forEach((tab) => {
  tab.addEventListener("click", () => setInstallMethod(tab.dataset.installTab));
});

copyButton.addEventListener("click", async () => {
  try {
    await navigator.clipboard.writeText(command.textContent);
    copyButton.textContent = "copied";
  } catch {
    copyButton.textContent = "select";
  }
  window.setTimeout(() => {
    copyButton.textContent = "copy";
  }, 1600);
});

setInstallMethod(detectInstallMethod());
