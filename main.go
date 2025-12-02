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

// --------------------
// AUTH
// --------------------
type Auth struct {
	UUID   string
	Secret string
}

func (a *Auth) GetRequestMetadata(ctx context.Context, uri ...string) (map[string]string, error) {
	return map[string]string{
		"uuid":   a.UUID,
		"secret": a.Secret,
	}, nil
}

func (a *Auth) RequireTransportSecurity() bool {
	return false
}

// --------------------
// UUID
// --------------------
func generateUUID() string {
	b := make([]byte, 8)
	_, _ = crand.Read(b)
	return hex.EncodeToString(b)
}

// --------------------
// Single Agent
// --------------------
func runAgent(server string, secret string) {
	uuid := generateUUID()

	auth := &Auth{UUID: uuid, Secret: secret}

	conn, err := grpc.NewClient(
		server,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithPerRPCCredentials(auth),
	)
	if err != nil {
		log.Println("connect error:", err)
		return
	}
	defer conn.Close()

	client := pb.NewNezhaServiceClient(conn)

	host := &pb.Host{
		Platform:        "SimulatedHost",
		PlatformVersion: "1.0",
		Cpu:             []string{"FakeCPU"},
		MemTotal:        4 * 1024 * 1024 * 1024,
		DiskTotal:       100 * 1024 * 1024 * 1024,
		SwapTotal:       0,
		Arch:            "amd64",
		Virtualization:  "kvm",
		BootTime:        uint64(time.Now().Unix()),
		Version:         uuid, // 用 UUID 作为唯一标识
		Gpu:             []string{},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err = client.ReportSystemInfo2(ctx, host)
	if err != nil {
		log.Println("report error:", err)
		return
	}

	log.Printf("[OK] Host %s 上报成功\n", uuid)
}

// --------------------
// MAIN
// --------------------
func main() {
	server := flag.String("server", "", "Nezha server address (ip:port)")
	secret := flag.String("secret", "", "Agent secret")
	count := flag.Int("count", 1, "number of agents to simulate")
	flag.Parse()

	if *server == "" || *secret == "" {
		log.Fatal("必须指定 --server 和 --secret")
	}

	log.Printf("模拟 %d 个 Agent -> %s", *count, *server)

	for i := 0; i < *count; i++ {
		go runAgent(*server, *secret)
	}

	select {}
}
