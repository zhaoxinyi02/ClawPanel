//go:build windows

package monitor

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	modKernel32                     = windows.NewLazySystemDLL("kernel32.dll")
	modAdvapi32                     = windows.NewLazySystemDLL("advapi32.dll")
	modWtsapi32                     = windows.NewLazySystemDLL("wtsapi32.dll")
	procOpenProcessToken            = modAdvapi32.NewProc("OpenProcessToken")
	procDuplicateTokenEx            = modAdvapi32.NewProc("DuplicateTokenEx")
	procSetTokenInformation         = modAdvapi32.NewProc("SetTokenInformation")
	procCreateProcessAsUserW        = modAdvapi32.NewProc("CreateProcessAsUserW")
	procWTSQuerySessionInformationW = modWtsapi32.NewProc("WTSQuerySessionInformationW")
	procWTSFreeMemory               = modWtsapi32.NewProc("WTSFreeMemory")
	modUserenv                      = windows.NewLazySystemDLL("userenv.dll")
	procCreateEnvironmentBlock      = modUserenv.NewProc("CreateEnvironmentBlock")
	procDestroyEnvironmentBlock     = modUserenv.NewProc("DestroyEnvironmentBlock")
)

const (
	TOKEN_DUPLICATE            = 0x0002
	TOKEN_QUERY                = 0x0008
	TOKEN_ASSIGN_PRIMARY       = 0x0001
	TOKEN_ADJUST_PRIVILEGES    = 0x0020
	TOKEN_ADJUST_SESSIONID     = 0x0100
	TOKEN_ADJUST_DEFAULT       = 0x0080
	SecurityImpersonation      = 2
	TokenPrimary               = 1
	TokenSessionId             = 12
	CREATE_UNICODE_ENVIRONMENT = 0x00000400
	NORMAL_PRIORITY_CLASS      = 0x00000020
	CREATE_NEW_CONSOLE         = 0x00000010
	CREATE_NO_WINDOW           = 0x08000000
)

type STARTUPINFOW struct {
	Cb              uint32
	LpReserved      *uint16
	LpDesktop       *uint16
	LpTitle         *uint16
	DwX             uint32
	DwY             uint32
	DwXSize         uint32
	DwYSize         uint32
	DwXCountChars   uint32
	DwYCountChars   uint32
	DwFillAttribute uint32
	DwFlags         uint32
	WShowWindow     uint16
	CbReserved2     uint16
	LpReserved2     *byte
	HStdInput       windows.Handle
	HStdOutput      windows.Handle
	HStdError       windows.Handle
}

type PROCESS_INFORMATION struct {
	HProcess    windows.Handle
	HThread     windows.Handle
	DwProcessId uint32
	DwThreadId  uint32
}

// launchAsInteractiveUser launches a process in the interactive user's session (session 1)
// by stealing the token from explorer.exe (which runs in session 1).
// cmdLine is the full quoted command line. extraEnv is appended to the user environment.
func launchAsInteractiveUser(cmdLine, workDir string, extraEnv []string) error {
	// Find explorer.exe in session 1
	explorerPID, err := findExplorerPID()
	if err != nil {
		return fmt.Errorf("find explorer.exe: %w", err)
	}
	log.Printf("[NapCat] Found explorer.exe PID=%d, stealing token for session-1 launch", explorerPID)

	// Open the explorer process
	hProcess, err := windows.OpenProcess(
		windows.PROCESS_QUERY_INFORMATION,
		false,
		explorerPID,
	)
	if err != nil {
		return fmt.Errorf("OpenProcess explorer: %w", err)
	}
	defer windows.CloseHandle(hProcess)

	// Open the process token
	var hToken windows.Token
	r, _, e := procOpenProcessToken.Call(
		uintptr(hProcess),
		TOKEN_DUPLICATE|TOKEN_QUERY,
		uintptr(unsafe.Pointer(&hToken)),
	)
	if r == 0 {
		return fmt.Errorf("OpenProcessToken: %w", e)
	}
	defer hToken.Close()

	// Duplicate the token as a primary token
	var hDupToken windows.Token
	r, _, e = procDuplicateTokenEx.Call(
		uintptr(hToken),
		TOKEN_ASSIGN_PRIMARY|TOKEN_DUPLICATE|TOKEN_QUERY|TOKEN_ADJUST_PRIVILEGES|TOKEN_ADJUST_SESSIONID|TOKEN_ADJUST_DEFAULT,
		0,
		SecurityImpersonation,
		TokenPrimary,
		uintptr(unsafe.Pointer(&hDupToken)),
	)
	if r == 0 {
		return fmt.Errorf("DuplicateTokenEx: %w", e)
	}
	defer hDupToken.Close()

	// Force the duplicated token into session 1 so the process actually runs there.
	sessionID := uint32(1)
	procSetTokenInformation.Call(
		uintptr(hDupToken),
		TokenSessionId,
		uintptr(unsafe.Pointer(&sessionID)),
		uintptr(unsafe.Sizeof(sessionID)),
	)

	// Create environment block for the user token.
	var rawEnvBlock *uint16
	r, _, e = procCreateEnvironmentBlock.Call(
		uintptr(unsafe.Pointer(&rawEnvBlock)),
		uintptr(hDupToken),
		0,
	)
	createEnvOK := r != 0 && rawEnvBlock != nil
	if !createEnvOK {
		log.Printf("[NapCat] CreateEnvironmentBlock failed, falling back to current process env baseline: %v", e)
	}

	// Build the final env block (user env merged with extraEnv overrides).
	var finalEnvBlock uintptr
	var mergedEnv []uint16
	if createEnvOK {
		if len(extraEnv) > 0 {
			mergedEnv = mergeEnvBlock(rawEnvBlock, extraEnv)
			finalEnvBlock = uintptr(unsafe.Pointer(&mergedEnv[0]))
		} else {
			finalEnvBlock = uintptr(unsafe.Pointer(rawEnvBlock))
		}
		defer procDestroyEnvironmentBlock.Call(uintptr(unsafe.Pointer(rawEnvBlock)))
	} else {
		mergedEnv = mergeEnvPairs(os.Environ(), extraEnv)
		finalEnvBlock = uintptr(unsafe.Pointer(&mergedEnv[0]))
	}

	// Build command line UTF16 pointer
	cmdLinePtr, err := windows.UTF16PtrFromString(cmdLine)
	if err != nil {
		return fmt.Errorf("UTF16PtrFromString cmdLine: %w", err)
	}
	wd, err := windows.UTF16PtrFromString(workDir)
	if err != nil {
		return fmt.Errorf("UTF16PtrFromString workDir: %w", err)
	}

	si := STARTUPINFOW{
		Cb: uint32(unsafe.Sizeof(STARTUPINFOW{})),
	}
	// Set desktop to winsta0\default (interactive desktop)
	desktop, _ := windows.UTF16PtrFromString(`winsta0\default`)
	si.LpDesktop = desktop

	var pi PROCESS_INFORMATION

	// Do NOT use CREATE_NO_WINDOW — NapCat needs desktop access for DLL injection into QQ.
	creationFlags := uint32(CREATE_UNICODE_ENVIRONMENT | NORMAL_PRIORITY_CLASS)

	r, _, e = procCreateProcessAsUserW.Call(
		uintptr(hDupToken),
		0,
		uintptr(unsafe.Pointer(cmdLinePtr)),
		0,
		0,
		0,
		uintptr(creationFlags),
		finalEnvBlock,
		uintptr(unsafe.Pointer(wd)),
		uintptr(unsafe.Pointer(&si)),
		uintptr(unsafe.Pointer(&pi)),
	)
	runtime.KeepAlive(mergedEnv)
	if r == 0 {
		return fmt.Errorf("CreateProcessAsUserW: %w", e)
	}

	windows.CloseHandle(pi.HProcess)
	windows.CloseHandle(pi.HThread)
	log.Printf("[NapCat] Launched in session 1 via CreateProcessAsUser (PID=%d): %s", pi.DwProcessId, cmdLine)
	return nil
}

// findExplorerPID returns the PID of explorer.exe running in session 1.
func findExplorerPID() (uint32, error) {
	snapshot, err := windows.CreateToolhelp32Snapshot(windows.TH32CS_SNAPPROCESS, 0)
	if err != nil {
		return 0, err
	}
	defer windows.CloseHandle(snapshot)

	var pe windows.ProcessEntry32
	pe.Size = uint32(unsafe.Sizeof(pe))
	if err := windows.Process32First(snapshot, &pe); err != nil {
		return 0, err
	}
	for {
		name := windows.UTF16ToString(pe.ExeFile[:])
		if name == "explorer.exe" {
			// Verify it's in session 1
			h, err := windows.OpenProcess(windows.PROCESS_QUERY_INFORMATION, false, pe.ProcessID)
			if err == nil {
				var sessionID uint32
				if err2 := windows.ProcessIdToSessionId(pe.ProcessID, &sessionID); err2 == nil && sessionID == 1 {
					windows.CloseHandle(h)
					return pe.ProcessID, nil
				}
				windows.CloseHandle(h)
			}
		}
		if err := windows.Process32Next(snapshot, &pe); err != nil {
			break
		}
	}
	return 0, fmt.Errorf("explorer.exe not found in session 1")
}

// mergeEnvBlock parses the Windows UTF-16 environment block at rawBlock,
// overrides/adds the key=value pairs from extraEnv, and returns a new
// UTF-16 double-null-terminated env block suitable for CreateProcessAsUserW.
func mergeEnvBlock(rawBlock *uint16, extraEnv []string) []uint16 {
	// Parse existing env block (null-separated UTF-16 pairs, double-null terminated)
	env := map[string]string{}
	if rawBlock != nil {
		ptr := unsafe.Pointer(rawBlock)
		for {
			// Read null-terminated UTF-16 string
			var chars []uint16
			for {
				w := *(*uint16)(ptr)
				ptr = unsafe.Add(ptr, unsafe.Sizeof(uint16(0)))
				if w == 0 {
					break
				}
				chars = append(chars, w)
			}
			if len(chars) == 0 {
				break // double-null: end of block
			}
			kv := windows.UTF16ToString(chars)
			if idx := strings.Index(kv, "="); idx > 0 {
				env[strings.ToUpper(kv[:idx])] = kv[idx+1:]
			}
		}
	}
	// Override with extra vars
	for _, kv := range extraEnv {
		if idx := strings.Index(kv, "="); idx > 0 {
			env[strings.ToUpper(kv[:idx])] = kv[idx+1:]
		}
	}
	// Rebuild block
	var out []uint16
	for k, v := range env {
		entry := k + "=" + v
		u16, _ := windows.UTF16FromString(entry)
		out = append(out, u16...) // includes null terminator from UTF16FromString
	}
	out = append(out, 0) // double-null terminator
	return out
}

func mergeEnvPairs(baseEnv, extraEnv []string) []uint16 {
	env := map[string]string{}
	for _, kv := range baseEnv {
		if idx := strings.Index(kv, "="); idx > 0 {
			env[strings.ToUpper(kv[:idx])] = kv[idx+1:]
		}
	}
	for _, kv := range extraEnv {
		if idx := strings.Index(kv, "="); idx > 0 {
			env[strings.ToUpper(kv[:idx])] = kv[idx+1:]
		}
	}

	var out []uint16
	for k, v := range env {
		entry := k + "=" + v
		u16, _ := windows.UTF16FromString(entry)
		out = append(out, u16...)
	}
	out = append(out, 0)
	return out
}

// launchNapCatInUserSession launches NapCat in the interactive user session.
// Strategy: write a PowerShell launcher script then run it via schtasks /RU <user>.
// We get the username via WTSQuerySessionInformationW (proper Unicode, no WMIC encoding issues).
// Fallback: CreateProcessAsUser with explorer.exe token.
func launchNapCatInUserSession(exePath, workDir string) error {
	batPath := findNapCatLauncherBat(workDir)
	if batPath == "" {
		return fmt.Errorf("launcher-user.bat not found in %s", workDir)
	}
	batDir := filepath.Dir(batPath)

	// Write a PS1 wrapper that runs the bat without pausing
	psContent := fmt.Sprintf(
		"$p = Start-Process -FilePath 'cmd.exe' -ArgumentList '/c \"%s\"' -WorkingDirectory '%s' -WindowStyle Hidden -PassThru; exit 0\r\n",
		batPath, batDir)
	psFile := filepath.Join(os.TempDir(), "napcat_launch.ps1")
	if err := os.WriteFile(psFile, []byte(psContent), 0644); err != nil {
		return fmt.Errorf("write ps1: %w", err)
	}

	taskCmd := fmt.Sprintf(`powershell.exe -NoProfile -ExecutionPolicy Bypass -WindowStyle Hidden -File "%s"`, psFile)

	// Try schtasks with WTS username first (proper Unicode)
	username := getSession1Username()
	taskName := "ClawPanelStartNapCat"
	exec.Command("schtasks", "/Delete", "/TN", taskName, "/F").Run()

	var createErr error
	if username != "" {
		log.Printf("[NapCat] schtasks /RU %s", username)
		createErr = exec.Command("schtasks", "/Create", "/F",
			"/TN", taskName, "/SC", "ONCE", "/ST", "00:00",
			"/RU", username, "/TR", taskCmd, "/RL", "HIGHEST",
		).Run()
	}
	if username == "" || createErr != nil {
		// Fallback: omit /RU (runs as SYSTEM but still triggers via schtasks)
		log.Printf("[NapCat] schtasks without /RU (username=%q, err=%v)", username, createErr)
		createErr = exec.Command("schtasks", "/Create", "/F",
			"/TN", taskName, "/SC", "ONCE", "/ST", "00:00",
			"/TR", taskCmd, "/RL", "HIGHEST",
		).Run()
	}
	if createErr != nil {
		// Last resort: CreateProcessAsUser
		log.Printf("[NapCat] schtasks create failed (%v), using CreateProcessAsUser", createErr)
		cmdLine, extraEnv, err := buildNapCatCommandLine(exePath, workDir)
		if err != nil {
			return fmt.Errorf("buildNapCatCommandLine: %w", err)
		}
		return launchAsInteractiveUser(cmdLine, workDir, extraEnv)
	}

	runErr := exec.Command("schtasks", "/Run", "/TN", taskName).Run()
	go func() {
		time.Sleep(20 * time.Second)
		exec.Command("schtasks", "/Delete", "/TN", taskName, "/F").Run()
		os.Remove(psFile)
	}()
	if runErr != nil {
		return fmt.Errorf("schtasks run: %w", runErr)
	}
	log.Printf("[NapCat] schtasks launched NapCat via %s", batPath)
	return nil
}

// getSession1Username returns the domain\username of the user logged into session 1
// using WTSQuerySessionInformationW which returns proper Unicode strings.
func getSession1Username() string {
	const WTSUserName = 5
	const WTSDomainName = 7
	const WTS_CURRENT_SERVER_HANDLE = 0

	getDomainUser := func(sessionID uint32, infoClass uintptr) string {
		var pBuf *uint16
		var bytes uint32
		r, _, _ := procWTSQuerySessionInformationW.Call(
			WTS_CURRENT_SERVER_HANDLE,
			uintptr(sessionID),
			infoClass,
			uintptr(unsafe.Pointer(&pBuf)),
			uintptr(unsafe.Pointer(&bytes)),
		)
		if r == 0 || pBuf == nil {
			return ""
		}
		defer procWTSFreeMemory.Call(uintptr(unsafe.Pointer(pBuf)))
		// pBuf points to a null-terminated UTF-16 string
		nChars := bytes / 2
		if nChars == 0 {
			return ""
		}
		u16 := unsafe.Slice(pBuf, nChars)
		return windows.UTF16ToString(u16)
	}

	domain := getDomainUser(1, WTSDomainName)
	user := getDomainUser(1, WTSUserName)
	if user == "" {
		return ""
	}
	if domain != "" && !strings.EqualFold(domain, ".") {
		return domain + `\` + user
	}
	return user
}

// findNapCatLauncherBat finds launcher-user.bat in the NapCat shell tree.
func findNapCatLauncherBat(shellDir string) string {
	// Check inner dir first (versions/.../napcat/)
	innerDir := findNapCatInnerDir(shellDir)
	if innerDir != "" {
		p := filepath.Join(innerDir, "launcher-user.bat")
		if _, err := os.Stat(p); err == nil {
			return p
		}
		p = filepath.Join(innerDir, "launcher.bat")
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	// Check top-level shell dir
	for _, name := range []string{"launcher-user.bat", "napcat.bat"} {
		p := filepath.Join(shellDir, name)
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	return ""
}

// buildNapCatCommandLine builds the full NapCatWinBootMain.exe command line with
// the QQ path argument and env vars, mimicking launcher-user.bat.
// napcatShellDir is the top-level NapCat.Shell dir. The inner resource dir
// (containing napcat.mjs, qqnt.json, and the actual NapCatWinBootMain.exe) is found by walking.
func buildNapCatCommandLine(_, napcatShellDir string) (cmdLine string, extraEnv []string, err error) {
	// Find QQ.exe path from registry (same as launcher.bat)
	qqPath := findQQExePath()
	if qqPath == "" {
		err = fmt.Errorf("QQ.exe not found in registry or common paths")
		return
	}
	log.Printf("[NapCat] Found QQ.exe at: %s", qqPath)

	// Find the inner napcat resource dir (where napcat.mjs lives)
	innerDir := findNapCatInnerDir(napcatShellDir)
	if innerDir == "" {
		innerDir = napcatShellDir // Fallback
	}
	log.Printf("[NapCat] Inner napcat dir: %s", innerDir)

	// Use the inner NapCatWinBootMain.exe (the actual injector, not the top-level stub)
	innerBootMain := filepath.Join(innerDir, "NapCatWinBootMain.exe")
	if _, e := os.Stat(innerBootMain); e != nil {
		// Fall back to shell dir
		innerBootMain = filepath.Join(napcatShellDir, "NapCatWinBootMain.exe")
	}
	log.Printf("[NapCat] Using boot main: %s", innerBootMain)

	// NAPCAT_MAIN_PATH uses forward slashes (as in the bat)
	napcatMjs := filepath.Join(innerDir, "napcat.mjs")
	napcatMjsFwd := strings.ReplaceAll(napcatMjs, `\`, `/`)

	// Write loadNapCat.js into the inner dir (same as the bat does with echo)
	loadPath := filepath.Join(innerDir, "loadNapCat.js")
	loadContent := fmt.Sprintf(`(async () => {await import("file:///%s")})()`, napcatMjsFwd)
	if werr := os.WriteFile(loadPath, []byte(loadContent), 0644); werr != nil {
		log.Printf("[NapCat] warning: could not write loadNapCat.js: %v", werr)
	}

	// Use inner NapCatWinBootHook.dll (larger, 20KB version in inner dir)
	injectDll := filepath.Join(innerDir, "NapCatWinBootHook.dll")
	if _, e := os.Stat(injectDll); e != nil {
		injectDll = filepath.Join(napcatShellDir, "NapCatWinBootHook.dll")
	}

	extraEnv = []string{
		"NAPCAT_PATCH_PACKAGE=" + filepath.Join(innerDir, "qqnt.json"),
		"NAPCAT_LOAD_PATH=" + loadPath,
		"NAPCAT_INJECT_PATH=" + injectDll,
		"NAPCAT_LAUNCHER_PATH=" + innerBootMain,
		"NAPCAT_MAIN_PATH=" + napcatMjs,
	}

	// Command line: "NapCatWinBootMain.exe" "D:\QQ\QQ.exe" "NapCatWinBootHook.dll"
	cmdLine = fmt.Sprintf(`"%s" "%s" "%s"`, innerBootMain, qqPath, injectDll)
	return
}

// findNapCatInnerDir finds the inner napcat resource directory (where napcat.mjs lives)
// by walking the NapCat Shell directory tree.
func findNapCatInnerDir(shellDir string) string {
	var found string
	filepath.WalkDir(shellDir, func(path string, d os.DirEntry, err error) error {
		if err != nil || found != "" {
			return nil
		}
		if !d.IsDir() && strings.EqualFold(d.Name(), "napcat.mjs") {
			found = filepath.Dir(path)
			return filepath.SkipAll
		}
		return nil
	})
	return found
}

// findQQExePath finds the QQ.exe path via registry, then common locations.
func findQQExePath() string {
	// Same registry key as launcher.bat
	out, err := exec.Command("reg", "query",
		`HKEY_LOCAL_MACHINE\SOFTWARE\WOW6432Node\Microsoft\Windows\CurrentVersion\Uninstall\QQ`,
		"/v", "UninstallString").Output()
	if err == nil {
		for _, line := range strings.Split(string(out), "\n") {
			line = strings.TrimSpace(line)
			if strings.Contains(strings.ToLower(line), "uninstallstring") {
				parts := strings.Fields(line)
				if len(parts) >= 3 {
					uninstStr := strings.Trim(parts[len(parts)-1], `"`)
					qqDir := filepath.Dir(uninstStr)
					qqExe := filepath.Join(qqDir, "QQ.exe")
					if _, e := os.Stat(qqExe); e == nil {
						return qqExe
					}
				}
			}
		}
	}
	// Fallback: common locations
	for _, p := range []string{`D:\QQ\QQ.exe`, `C:\Program Files\Tencent\QQ\QQ.exe`, `C:\QQ\QQ.exe`} {
		if _, e := os.Stat(p); e == nil {
			return p
		}
	}
	return ""
}

// launchNapCatViaBat launches NapCat via the launcher-user.bat as a fallback.
func launchNapCatViaBat(napcatDir string) error {
	batPath := filepath.Join(napcatDir, "launcher-user.bat")
	if _, err := os.Stat(batPath); err != nil {
		// Try the top-level napcat.bat
		batPath = filepath.Join(filepath.Dir(napcatDir), "napcat.bat")
		if _, err2 := os.Stat(batPath); err2 != nil {
			return fmt.Errorf("no launcher bat found")
		}
	}
	cmdLine := fmt.Sprintf(`cmd.exe /c "%s"`, batPath)
	return launchAsInteractiveUser(cmdLine, napcatDir, nil)
}

// launchNapCatViaSchtasks is the schtasks fallback for launchNapCatInUserSession.
func launchNapCatViaSchtasks(cmdLine, workDir string) error {
	psContent := fmt.Sprintf("Set-Location '%s'\n%s\n", workDir, cmdLine)
	psFile := filepath.Join(os.TempDir(), "napcat_launch.ps1")
	_ = os.WriteFile(psFile, []byte(psContent), 0644)

	username := getInteractiveUsername()
	taskName := "ClawPanelStartNapCat"
	tr := fmt.Sprintf(`powershell.exe -NoProfile -ExecutionPolicy Bypass -WindowStyle Hidden -File "%s"`, psFile)
	exec.Command("schtasks", "/Delete", "/TN", taskName, "/F").Run()
	var createArgs []string
	createArgs = []string{"/Create", "/F", "/TN", taskName, "/SC", "ONCE", "/ST", "00:00", "/TR", tr, "/RL", "HIGHEST"}
	if username != "" {
		createArgs = append(createArgs, "/RU", username)
	}
	err := exec.Command("schtasks", createArgs...).Run()
	if err == nil {
		if err = exec.Command("schtasks", "/Run", "/TN", taskName).Run(); err == nil {
			go func() {
				time.Sleep(15 * time.Second)
				exec.Command("schtasks", "/Delete", "/TN", taskName, "/F").Run()
				os.Remove(psFile)
			}()
			return nil
		}
	}
	return fmt.Errorf("schtasks failed: %w", err)
}
