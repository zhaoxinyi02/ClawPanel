package main

import (
	"embed"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

//go:embed payload/*
var payload embed.FS

const (
	SERVICE_NAME = "ClawPanel"
	PORT         = "19527"
)

var VERSION = "5.0.0"

func main() {
	printBanner()

	if runtime.GOOS == "windows" {
		windowsInstall()
	} else {
		fmt.Println("  此安装程序仅支持 Windows。")
		fmt.Println("  Linux/macOS 请使用一键安装脚本。")
		pause()
	}
}

func windowsInstall() {
	installDir := `C:\ClawPanel`
	binaryName := "clawpanel.exe"

	// 检查管理员权限 - 尝试写入 Windows 目录
	testFile := filepath.Join(os.Getenv("SystemRoot"), ".clawpanel_test")
	if err := os.WriteFile(testFile, []byte("test"), 0644); err != nil {
		fmt.Println()
		fmt.Println("  [错误] 请右键选择「以管理员身份运行」此安装程序！")
		fmt.Println()
		pause()
		os.Exit(1)
	}
	os.Remove(testFile)

	fmt.Println("  [1/5] 创建安装目录...")
	os.MkdirAll(installDir, 0755)
	os.MkdirAll(filepath.Join(installDir, "data"), 0755)
	fmt.Printf("        %s\n", installDir)

	fmt.Println("  [2/5] 释放 ClawPanel 主程序...")
	targetPath := filepath.Join(installDir, binaryName)
	// 先停止旧服务
	runCmd("sc", "stop", SERVICE_NAME)
	// 等一下让服务停止
	runCmd("timeout", "/t", "2", "/nobreak")

	data, err := fs.ReadFile(payload, "payload/"+binaryName)
	if err != nil {
		fmt.Printf("  [错误] 读取内嵌文件失败: %v\n", err)
		fmt.Println("  请确保安装包完整。")
		pause()
		os.Exit(1)
	}
	if err := os.WriteFile(targetPath, data, 0755); err != nil {
		fmt.Printf("  [错误] 写入文件失败: %v\n", err)
		pause()
		os.Exit(1)
	}
	fmt.Printf("        已释放 %s (%.1f MB)\n", targetPath, float64(len(data))/1024/1024)

	fmt.Println("  [3/5] 注册 Windows 服务（开机自启动）...")
	// 删除旧服务
	runCmd("sc", "delete", SERVICE_NAME)
	// 创建新服务
	binPathArg := fmt.Sprintf("binPath=%s", targetPath)
	out := runCmdOutput("sc", "create", SERVICE_NAME, binPathArg, "start=auto",
		fmt.Sprintf("DisplayName=ClawPanel v%s", VERSION))
	if strings.Contains(out, "SUCCESS") || strings.Contains(out, "成功") {
		fmt.Println("        服务已注册，开机自启动已启用")
	} else {
		fmt.Printf("        %s\n", strings.TrimSpace(out))
	}
	// 设置描述
	runCmd("sc", "description", SERVICE_NAME,
		fmt.Sprintf("ClawPanel v%s - OpenClaw 智能助手管理面板", VERSION))
	// 设置失败自动重启
	runCmd("sc", "failure", SERVICE_NAME,
		"reset=86400", "actions=restart/5000/restart/10000/restart/30000")

	fmt.Println("  [4/5] 配置防火墙规则...")
	runCmd("netsh", "advfirewall", "firewall", "delete", "rule", "name=ClawPanel")
	fwOut := runCmdOutput("netsh", "advfirewall", "firewall", "add", "rule",
		"name=ClawPanel", "dir=in", "action=allow", "protocol=TCP",
		fmt.Sprintf("localport=%s", PORT))
	if strings.Contains(fwOut, "Ok") || strings.Contains(fwOut, "确定") {
		fmt.Printf("        已放行端口 %s\n", PORT)
	}

	fmt.Println("  [5/5] 启动 ClawPanel 服务...")
	startOut := runCmdOutput("sc", "start", SERVICE_NAME)
	if strings.Contains(startOut, "RUNNING") || strings.Contains(startOut, "START_PENDING") {
		fmt.Println("        服务已启动")
	} else {
		fmt.Printf("        %s\n", strings.TrimSpace(startOut))
		fmt.Println("        可手动启动: sc start ClawPanel")
	}

	printSuccess(installDir)
	pause()
}

func printBanner() {
	fmt.Println()
	fmt.Println("  ============================================================")
	fmt.Println("                ClawPanel v" + VERSION + " 安装程序")
	fmt.Println("         OpenClaw 智能管理面板 - 单文件部署版")
	fmt.Println("  ============================================================")
	fmt.Println()
}

func printSuccess(installDir string) {
	fmt.Println()
	fmt.Println("  ============================================================")
	fmt.Println("         ClawPanel v" + VERSION + " 安装完成!")
	fmt.Println("  ============================================================")
	fmt.Println()
	fmt.Println("  访问地址: http://localhost:" + PORT)
	fmt.Println("  默认密码: clawpanel")
	fmt.Println()
	fmt.Println("  安装目录: " + installDir)
	fmt.Println("  数据目录: " + installDir + "\\data")
	fmt.Println()
	fmt.Println("  服务管理:")
	fmt.Println("    sc start ClawPanel   - 启动")
	fmt.Println("    sc stop ClawPanel    - 停止")
	fmt.Println("    sc query ClawPanel   - 查看状态")
	fmt.Println()
	fmt.Println("  !! 请登录后立即修改默认密码 !!")
	fmt.Println()
}

func runCmd(name string, args ...string) {
	cmd := exec.Command(name, args...)
	cmd.Stdout = nil
	cmd.Stderr = nil
	cmd.Run()
}

func runCmdOutput(name string, args ...string) string {
	cmd := exec.Command(name, args...)
	out, _ := cmd.CombinedOutput()
	return string(out)
}

func pause() {
	fmt.Print("  按回车键退出...")
	var input string
	fmt.Scanln(&input)
}
