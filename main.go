package main

import (
	"context"
	crand "crypto/rand"
	"encoding/hex"
	"flag"
	"log"
	"time"

	pb "github.com/nezhahq/agent/proto"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func generateUUID() string {
	b := make([]byte, 8)
	_, _ = crand.Read(b)
	return hex.EncodeToString(b)
}

func runAgent(server, secret string) {
	// gRPC 连接
	conn, err := grpc.NewClient(
		server,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		log.Println("connect error:", err)
		return
	}
	defer conn.Close()

	client := pb.NewNezhaServiceClient(conn)

	// 生成随机 UUID（仅代表一台主机）
	uuid := generateUUID()

	// 上报 Host 信息
	host := &pb.Host{
		Platform:        "SimulatedHost",
		PlatformVersion: "1.0",
		Cpu:             []string{"Fake CPU"},
		MemTotal:        4 * 1024 * 1024 * 1024,     // 4GB
		DiskTotal:       100 * 1024 * 1024 * 1024,   // 100GB
		SwapTotal:       0,
		Arch:            "amd64",
		Virtualization:  "kvm",
		BootTime:        uint64(time.Now().Unix()),  // 当前时间
		Version:         uuid,                       // 用 uuid 当作“唯一识别符”
		Gpu:             []string{},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err = client.ReportSystemInfo2(ctx, host)
	if err != nil {
		log.Println("report error:", err)
		return
	}

	log.Printf("Sim Host %s 上报成功\n", uuid)
}

func main() {
	server := flag.String("server", "", "nezha server address (ip:port)")
	secret := flag.String("secret", "", "agent secret")
	count := flag.Int("count", 1, "number of agents to simulate")
	flag.Parse()

	if *server == "" || *secret == "" {
		log.Fatal("必须指定 --server 和 --secret")
	}

	log.Printf("启动模拟器: %d 个 Agent -> %s", *count, *server)

	for i := 0; i < *count; i++ {
		go runAgent(*server, *secret)
	}

	select {} // 永不退出
}
