package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"

	// TODO: 请将此路径替换为您项目中编译后的 Nezha Protocol Buffer 导入路径
	// 这是一个示例路径，您需要根据实际情况修改
	pb "your/path/to/nezha/agent/proto" 
)

// --- 配置变量 (由命令行参数填充) ---
var (
	serverAddr string
	secret     string
	numAgents  int
)

// --- 1. 模拟认证凭证结构 (PerRPCCredentials) ---

type AuthHandler struct {
	ClientSecret string
	ClientUUID   string
}

func (a *AuthHandler) GetRequestMetadata(ctx context.Context, uri ...string) (map[string]string, error) {
	return map[string]string{
		"uuid":   a.ClientUUID,
		"secret": a.ClientSecret,
	}, nil
}

func (a *AuthHandler) RequireTransportSecurity() bool {
	return false 
}

// --- 2. 模拟主机信息生成函数 ---

// generateSimulatedHostInfo 模拟生成主机信息
func generateSimulatedHostInfo(uuid string) *pb.Host {
	return &pb.Host{
		Platform:      fmt.Sprintf("Simulated Server %s", uuid),
		PlatformVersion: "v1.0",
		Uptime:        uint64(time.Now().Unix()),
		Arch:          "sim_arch", 
		Virtualization: "vmware",
		Cpu:           &pb.CPU{
			Model: "Simulated CPU Model",
			Cores: 8,
		},
		Mem:           &pb.Mem{
			Total: 16 * 1024 * 1024 * 1024, // 16 GB
		},
		Disk:          &pb.Disk{
			Total: 512 * 1024 * 1024 * 1024, // 512 GB
		},
		Swap: &pb.Swap{
			Total: 4 * 1024 * 1024 * 1024,
		},
	}
}

// --- 3. Agent 客户端主运行逻辑 ---

func runSimulatedAgent(serverAddr, secret, uuid string, useTLS bool) {
	fmt.Printf("Agent %s: 开始连接 [%s]...\n", uuid, serverAddr)
	
	auth := &AuthHandler{
		ClientSecret: secret,
		ClientUUID:   uuid,
	}

	var dialOptions []grpc.DialOption
	dialOptions = append(dialOptions, grpc.WithPerRPCCredentials(auth))

	if useTLS {
		dialOptions = append(dialOptions, grpc.WithTransportCredentials(credentials.NewTLS(nil))) 
	} else {
		dialOptions = append(dialOptions, grpc.WithTransportCredentials(insecure.NewCredentials()))
	}
	
	conn, err := grpc.NewClient(serverAddr, dialOptions...)
	if err != nil {
		fmt.Printf("Agent %s [连接失败]: %v\n", uuid, err)
		return
	}
	defer conn.Close()

	client := pb.NewNezhaServiceClient(conn)
	
	// --- 仅执行一次：上报主机信息 (ReportSystemInfo2) ---
	
	hostInfo := generateSimulatedHostInfo(uuid)
	
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	
	_, err = client.ReportSystemInfo2(ctx, hostInfo)
	if err != nil {
		fmt.Printf("Agent %s [上报失败]: %v\n", uuid, err)
	} else {
		fmt.Printf("Agent %s [上报成功]: 主机信息已发送。\n", uuid)
	}
}

// --- 4. 命令行解析及运行 ---

func init() {
	// 定义命令行参数
	flag.StringVar(&serverAddr, "ip", "", "Nezha 服务器地址和端口 (e.g., 127.0.0.1:5555)")
	flag.StringVar(&serverAddr, "ip", "", "Nezha 服务器地址和端口 (e.g., 127.0.0.1:5555)")
	flag.StringVar(&secret, "k", "", "Nezha Agent 秘钥 (Client Secret)")
	flag.IntVar(&numAgents, "n", 1, "模拟 Agent 的数量")
}

func main() {
	flag.Parse()

	// 验证必要参数
	if serverAddr == "" || secret == "" {
		fmt.Println("❌ 错误: 必须指定服务器地址 (-ip) 和秘钥 (-k)。")
		flag.Usage()
		os.Exit(1)
	}
	if numAgents <= 0 {
		fmt.Println("❌ 错误: 模拟 Agent 数量 (-n) 必须大于 0。")
		os.Exit(1)
	}

	fmt.Printf("✅ 开始模拟 %d 台 Agent (一次性上报)...\n", numAgents)
	fmt.Printf("   目标服务器: %s\n", serverAddr)
	
	// 简单检查地址中是否包含 TLS 端口 (例如 443)，但最好通过配置明确指定
	useTLS := false 
	if len(serverAddr) > 0 && (serverAddr[len(serverAddr)-4:] == ":443" || serverAddr[len(serverAddr)-3:] == ":443") {
		useTLS = true
		fmt.Println("   警告: 检测到 443 端口，尝试使用 TLS 连接。")
	}

	for i := 1; i <= numAgents; i++ {
		simulatedUUID := fmt.Sprintf("SIM-AGENT-%03d", i)
		
		// 使用 Goroutine 并发运行 Agent
		go runSimulatedAgent(serverAddr, secret, simulatedUUID, useTLS)
	}

	// 等待足够的时间确保所有 gRPC 调用完成
	fmt.Println("\n等待 10 秒，确保所有 Agent 完成上报并退出...")
	time.Sleep(10 * time.Second)
    
    fmt.Println("所有模拟 Agent 已完成上报。程序退出。")
}
