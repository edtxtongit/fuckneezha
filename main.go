package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"flag"
	"fmt"
	"log"
	"time"

	pb "github.com/nezhahq/agent/proto" // ← 修改

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

var (
	server = flag.String("server", "127.0.0.1:5555", "dashboard IP:PORT")
	secret = flag.String("secret", "", "Nezha dashboard secret")
	count  = flag.Int("n", 1, "Simulated machines number")
)

func randomUUID() string {
	buf := make([]byte, 16)
	rand.Read(buf)
	return hex.EncodeToString(buf)
}

func simulateAgent(id int) {
	uuid := randomUUID()
	log.Printf("[Agent %d] UUID = %s", id, uuid)

	conn, err := grpc.Dial(*server,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithPerRPCCredentials(&tokenAuth{Token: *secret}),
	)
	if err != nil {
		log.Fatalf("connect error: %v", err)
	}
	defer conn.Close()

	client := pb.NewNezhaServiceClient(conn)

	// 1. ReportSystemInfo
	_, err = client.ReportSystemInfo(context.Background(), &pb.Host{
		Platform:         "linux",
		PlatformVersion:  "5.10",
		Cpu:              []string{"Intel(R) Fake CPU"},
		MemTotal:         2048 * 1024 * 1024,
		DiskTotal:        20 * 1024 * 1024 * 1024,
		Arch:             "amd64",
		Virtualization:   "kvm",
		BootTime:         uint64(time.Now().Unix() - 1000),
		Version:          "0.16.0",
	})
	if err != nil {
		log.Printf("[Agent %d] report info err: %v", id, err)
		return
	}

	// 2. ReportSystemState (持续上报)
	go func() {
		stream, _ := client.ReportSystemState(context.Background())
		for {
			stream.Send(&pb.State{
				Cpu:            3.1,
				MemUsed:        1200 * 1024 * 1024,
				DiskUsed:       10 * 1024 * 1024 * 1024,
				NetInSpeed:     1000,
				NetOutSpeed:    2000,
				Uptime:         uint64(time.Now().Unix()),
				Load1:          0.2,
				TcpConnCount:   10,
				ProcessCount:   60,
			})
			time.Sleep(8 * time.Second)
		}
	}()

	select {}
}

type tokenAuth struct{ Token string }

func (t *tokenAuth) GetRequestMetadata(context.Context, ...string) (map[string]string, error) {
	return map[string]string{"Token": t.Token}, nil
}
func (t *tokenAuth) RequireTransportSecurity() bool { return false }

func main() {
	flag.Parse()

	for i := 1; i <= *count; i++ {
		go simulateAgent(i)
	}

	select {}
}
