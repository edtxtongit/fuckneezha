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

	// 修改为你本地 Nezha proto 的真实路径
	pb "your/path/to/nezha/agent/proto"
)

// CLI 参数
var (
	serverAddr string
	secret     string
	numAgents  int
)

// --- 模拟认证 (PerRPCCredentials) ---
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

func (a *AuthHandler) RequireTransportSecurity() bool { return false }

// --- 生成模拟机器信息 ---
func generateSimulatedHostInfo(uuid string) *pb.Host {
	return &pb.Host{
		Platform:        fmt.Sprintf("Simulated-%s", uuid),
		PlatformVersion: "1.0",
		Uptime:          uint64(time.Now().Unix()),
		Arch:            "amd64",
		Virtualization:  "vmware",
		Cpu: &pb.CPU{
			Model: "Fake CPU",
			Cores: 8,
		},
		Mem: &pb.Mem{
			Total: 16384 * 1024 * 1024,
		},
		Disk: &pb.Disk{
			Total: 512 * 1024 * 1024 * 1024,
		},
		Swap: &pb.Swap{
			Total: 4 * 1024 * 1024 * 1024,
		},
	}
}

// --- 单个 Agent 的运行逻辑 ---
func runSimulatedAgent(serverAddr, secret, uuid string, useTLS bool) {
	fmt.Printf("[Agent %s] connecting to %s ...\n", uuid, serverAddr)

	auth := &AuthHandler{
		ClientSecret: secret,
		ClientUUID:   uuid,
	}

	// gRPC Dial Options
	var opts []grpc.DialOption
	opts = append(opts, grpc.WithPerRPCCredentials(auth))

	if useTLS {
		opts = append(opts, grpc.WithTransportCredentials(credentials.NewTLS(nil)))
	} else {
		opts = append(opts, grpc.WithTransportCredentials(insecure.NewCredentials()))
	}

	// --- 使用新版 API：grpc.NewClient ---
	conn, err := grpc.NewClient(serverAddr, opts...)
	if err != nil {
		fmt.Printf("[Agent %s] connect failed: %v\n", uuid, err)
		return
	}
	defer conn.Close()

	client := pb.NewNezhaServiceClient(conn)

	// 上报信息
	hostInfo := generateSimulatedHostInfo(uuid)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err = client.ReportSystemInfo2(ctx, hostInfo)
	if err != nil {
		fmt.Printf("[Agent %s] report failed: %v\n", uuid, err)
		return
	}

	fmt.Printf("[Agent %s] report OK\n", uuid)
}

// --- main ---
func init() {
	flag.StringVar(&serverAddr, "ip", "", "Nezha Server address (e.g. 127.0.0.1:5555)")
	flag.StringVar(&secret, "k", "", "Agent Secret")
	flag.IntVar(&numAgents, "n", 1, "How many agents to simulate")
}

func main() {
	flag.Parse()

	if serverAddr == "" || secret == "" {
		fmt.Println("ERROR: -ip and -k are required.")
		flag.Usage()
		return
	}

	useTLS := false
	if len(serverAddr) >= 4 && serverAddr[len(serverAddr)-4:] == ":443" {
		useTLS = true
	}

	fmt.Printf("Simulating %d agents ...\n", numAgents)

	for i := 1; i <= numAgents; i++ {
		uid := fmt.Sprintf("SIM-%03d", i)
		go runSimulatedAgent(serverAddr, secret, uid, useTLS)
	}

	time.Sleep(10 * time.Second)
	fmt.Println("All agents finished.")
}
