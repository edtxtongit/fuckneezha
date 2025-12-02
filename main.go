package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"flag"
	"log"
	"math/rand"
	"time"

	pb "github.com/nezhahq/agent/proto"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func randomUUID() string {
	b := make([]byte, 16)
	rand.Read(b)
	return hex.EncodeToString(b)
}

func runAgent(serverAddr, secret string, id int) {
	// 连接 Nezha 服务端
	conn, err := grpc.Dial(serverAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Printf("[A-%d] dial error: %v", id, err)
		return
	}
	defer conn.Close()

	client := pb.NewNezhaServiceClient(conn)

	ctx := context.Background()

	// ----------------------------
	// 1) ReportSystemInfo (Host)
	// ----------------------------
	host := &pb.Host{
		Platform:        "linux",
		PlatformVersion: "5.15",
		Cpu:             []string{"Intel(R) Xeon(R)", "FakeCore"},
		MemTotal:        2048 * 1024 * 1024,
		DiskTotal:       20 * 1024 * 1024 * 1024,
		SwapTotal:       0,
		Arch:            "amd64",
		Virtualization:  "kvm",
		BootTime:        uint64(time.Now().Unix() - 1000),
		Version:         "0.0.0-sim",
		Gpu:             []string{},
	}

	_, err = client.ReportSystemInfo(ctx, host)
	if err != nil {
		log.Printf("[A-%d] ReportSystemInfo failed: %v", id, err)
		return
	}

	log.Printf("[A-%d] Connected OK", id)

	// ----------------------------
	// 2) ReportSystemState (stream)
	// ----------------------------
	stream, err := client.ReportSystemState(ctx)
	if err != nil {
		log.Printf("[A-%d] ReportSystemState error: %v", id, err)
		return
	}

	// 异步接收服务器响应
	go func() {
		for {
			_, err := stream.Recv()
			if err != nil {
				return
			}
		}
	}()

	r := rand.New(rand.NewSource(time.Now().UnixNano() + int64(id)))

	for {
		state := &pb.State{
			Cpu:           r.Float64() * 80,
			MemUsed:       r.Uint64()%host.MemTotal + 100*1024*1024,
			SwapUsed:      0,
			DiskUsed:      r.Uint64()%host.DiskTotal + 1*1024*1024*1024,
			NetInTransfer: r.Uint64() % 1000000,
			NetOutTransfer:r.Uint64() % 1000000,
			NetInSpeed:    r.Uint64() % 300000,
			NetOutSpeed:   r.Uint64() % 300000,
			Uptime:        uint64(time.Now().Unix() - int64(host.BootTime)),
			Load1:         r.Float64(),
			Load5:         r.Float64(),
			Load15:        r.Float64(),
			TcpConnCount:  r.Uint64() % 200,
			UdpConnCount:  r.Uint64() % 50,
			ProcessCount:  r.Uint64()%300 + 10,
			Gpu:           []float64{},
		}

		err = stream.Send(state)
		if err != nil {
			log.Printf("[A-%d] stream send error: %v", id, err)
			return
		}

		time.Sleep(1 * time.Second)
	}
}

func main() {
	server := flag.String("server", "127.0.0.1:5555", "nezha server address")
	secret := flag.String("secret", "", "agent secret")
	count := flag.Int("count", 1, "number of agents")
	flag.Parse()

	if *secret == "" {
		log.Fatal("secret cannot be empty")
	}

	log.Printf("Starting %d agents -> %s", *count, *server)

	for i := 0; i < *count; i++ {
		go runAgent(*server, *secret, i)
	}

	select {} // 永不退出
}
