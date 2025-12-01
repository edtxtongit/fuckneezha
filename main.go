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

	pb "github.com/nezhahq/agent/proto"
)

// --- CLI 参数 ---
var (
	serverAddr string
	secret     string
	numAgents  int
)

// --- gRPC 鉴权（与 Nezha Agent 一致） ---
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

// --- 生成模拟 Host 信息（完全适配新版 proto） ---
func generateSimulatedHostInfo(uuid string) *pb.Host {
	return &pb.Host{
		Hostname:        "Simulated-" + uuid,
		Platform:        "SimOS",
		PlatformVersion: "1.0",
		KernelVersion:   "5.15-sim",
		CPUModel:        "Simulated CPU",
		CPUMhz:          2400,
		CPUCores:        8,
		MemoryTotal:     16 * 1024 * 1024 * 1024,  // 16GB
		DiskTotal:       512 * 1024 * 1024 * 1024, // 512GB
		Arch:            "x86_64",
		Virtualization:  "vmware",
		IpAddresses:     []string{"192.168.1.100"},
		BootTime:        uint64(time.Now().Add(-1 * time.Hour).Unix()),
	}
}

// --- 模拟 Agent 运行 ---
func runSimulatedAgent(serverAddr, secret, uuid string, useTLS bool) {
	fmt.Printf("Agent %s: connecting to %s...\n", uuid, serverAddr)

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

	conn, err := grpc.Dial(serverAddr, dialOptions...)
	if err != nil {
		fmt.Printf("Agent %s connect failed: %v\n", uuid, err)
		return
	}
	defer conn.Close()

	client := pb.NewAgentServiceClient(conn)

	host := generateSimulatedHostInfo(uuid)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err = client.ReportHostInfo(ctx, host)
	if err != nil {
		fmt.Printf("Agent %s report failed: %v\n", uuid, err)
	} else {
		fmt.Printf("Agent %s report OK\n", uuid)
	}
}

func init() {
	flag.StringVar(&serverAddr, "ip", "", "Nezha server address (e.g. 1.2.3.4:5555)")
	flag.StringVar(&secret, "k", "", "Agent secret")
	flag.IntVar(&numAgents, "n", 1, "Number of simulated agents")
}

func main() {
	flag.Parse()

	if serverAddr == "" || secret == "" {
		fmt.Println("Usage: -ip 1.2.3.4:5555 -k SECRET -n 10")
		os.Exit(1)
	}

	fmt.Printf("Simulating %d agents to %s\n", numAgents, serverAddr)

	useTLS := false
	if len(serverAddr) > 4 && serverAddr[len(serverAddr)-4:] == ":443" {
		useTLS = true
		fmt.Println("Detected port 443, using TLS")
	}

	for i := 1; i <= numAgents; i++ {
		uuid := fmt.Sprintf("SIM-%03d", i)
		go runSimulatedAgent(serverAddr, secret, uuid, useTLS)
	}

	time.Sleep(10 * time.Second)
	fmt.Println("All simulated agents done.")
}
