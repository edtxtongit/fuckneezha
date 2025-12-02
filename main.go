package main

import (
	"context"
	"crypto/tls"
	"flag"
	"log"
	"time"

	pb "github.com/nezhahq/agent/proto"

	"github.com/hashicorp/go-uuid"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
)

//
// ----- Per-RPC Auth -----
//

type Auth struct {
	ClientSecret string
	ClientUUID   string
}

func (a *Auth) GetRequestMetadata(ctx context.Context, uri ...string) (map[string]string, error) {
	return map[string]string{
		"client_secret": a.ClientSecret,
		"client_uuid":   a.ClientUUID,
	}, nil
}

func (a *Auth) RequireTransportSecurity() bool {
	return false
}

func genUUID() string {
	id, _ := uuid.GenerateUUID()
	return id
}

//
// ----- Single-Shot Agent（只上报一次） -----
//

func runAgent(server, secret string, idx int, useTLS bool) {
	uuid := genUUID()
	auth := &Auth{ClientSecret: secret, ClientUUID: uuid}

	// 连接
	var transport grpc.DialOption
	if useTLS {
		transport = grpc.WithTransportCredentials(credentials.NewTLS(&tls.Config{MinVersion: tls.VersionTLS12}))
	} else {
		transport = grpc.WithTransportCredentials(insecure.NewCredentials())
	}

	conn, err := grpc.Dial(server,
		transport,
		grpc.WithBlock(),
		grpc.WithPerRPCCredentials(auth),
		grpc.FailOnNonTempDialError(true),
		grpc.WithTimeout(10*time.Second),
	)
	if err != nil {
		log.Printf("[A-%d] dial err: %v", idx, err)
		return
	}
	defer conn.Close()

	client := pb.NewNezhaServiceClient(conn)

	// ---- ReportSystemInfo2（只上报一次） ----

	host := &pb.Host{
		Platform:        "linux",
		PlatformVersion: "5.15",
		Cpu:             []string{"Fake-CPU"},
		MemTotal:        4 * 1024 * 1024 * 1024,
		DiskTotal:       50 * 1024 * 1024 * 1024,
		Arch:            "amd64",
		Virtualization:  "kvm",
		BootTime:        uint64(time.Now().Unix()),
		Version:         "1.14.9-single",
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	_, err = client.ReportSystemInfo2(ctx, host)
	cancel()
	if err != nil {
		log.Printf("[A-%d] ReportSystemInfo2 err: %v", idx, err)
		return
	}

	// ---- ReportSystemState 流（只发送一次） ----

	stream, err := client.ReportSystemState(context.Background())
	if err != nil {
		log.Printf("[A-%d] open stream err: %v", idx, err)
		return
	}

	state := &pb.State{
		Cpu:          1.5,
		MemUsed:      512 * 1024 * 1024,
		Load1:        0.2,
		Load5:        0.1,
		Load15:       0.05,
		ProcessCount: 20,
		Uptime:       uint64(time.Now().Unix()),
	}

	if err := stream.Send(state); err != nil {
		log.Printf("[A-%d] state send err: %v", idx, err)
		return
	}

	// 发送完成后立即 CloseSend
	stream.CloseSend()

	log.Printf("[A-%d][%s] 完成一次认证 + 一次上报", idx, uuid)
}

//
// ----- Main -----
//

func main() {
	server := flag.String("server", "", "x.x.x.x:5555")
	secret := flag.String("secret", "", "agent secret")
	count := flag.Int("count", 1, "spawn N agents")
	tlsFlag := flag.Bool("tls", false, "use TLS")
	flag.Parse()

	if *server == "" || *secret == "" {
		log.Fatal("--server --secret 必须填写")
	}

	log.Printf("启动 %d 个一次性 agent\n", *count)

	for i := 0; i < *count; i++ {
		go runAgent(*server, *secret, i, *tlsFlag)
	}

	select {}
}
