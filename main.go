package main

import (
	"context"
	crand "crypto/rand"
	"encoding/hex"
	"flag"
	"log"
	"math/rand"
	"time"

	pb "github.com/nezhahq/agent/proto"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
)

// Auth implements PerRPCCredentials sending clientSecret + clientUUID (matches agent source)
type Auth struct {
	ClientSecret string
	ClientUUID   string
}

func (a *Auth) GetRequestMetadata(ctx context.Context, uri ...string) (map[string]string, error) {
	return map[string]string{
		"clientSecret": a.ClientSecret,
		"clientUUID":   a.ClientUUID,
	}, nil
}

func (a *Auth) RequireTransportSecurity() bool {
	// original agent returns false; keep false to allow insecure testing.
	// If you run in production with TLS, pass --tls and server will accept secure transport.
	return false
}

func genUUID() string {
	b := make([]byte, 8)
	_, _ = crand.Read(b)
	return hex.EncodeToString(b)
}

func runAgent(server string, secret string, idx int, useTLS bool) {
	uuid := genUUID()
	auth := &Auth{
		ClientSecret: secret,
		ClientUUID:   uuid,
	}

	// choose transport credentials
	var transport grpc.DialOption
	if useTLS {
		transport = grpc.WithTransportCredentials(credentials.NewTLS(nil))
	} else {
		transport = grpc.WithTransportCredentials(insecure.NewCredentials())
	}

	// dial with per-RPC creds
	ctxDial, cancelDial := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancelDial()

	conn, err := grpc.DialContext(ctxDial, server,
		transport,
		grpc.WithPerRPCCredentials(auth),
		grpc.WithBlock(),
	)
	if err != nil {
		log.Printf("[A-%d][%s] dial error: %v", idx, uuid, err)
		return
	}
	defer conn.Close()

	client := pb.NewNezhaServiceClient(conn)

	// also include uuid in outgoing ctx metadata for compatibility
	ctx := context.Background()
	ctx = context.WithValue(ctx, "uuid", uuid) // not used by gRPC metadata but kept for clarity

	// Build Host per proto you provided; Version field uses uuid for uniqueness
	host := &pb.Host{
		Platform:        "linux",
		PlatformVersion: "5.15",
		Cpu:             []string{"FakeCPU"},
		MemTotal:        4 * 1024 * 1024 * 1024,
		DiskTotal:       100 * 1024 * 1024 * 1024,
		SwapTotal:       0,
		Arch:            "amd64",
		Virtualization:  "kvm",
		BootTime:        uint64(time.Now().Unix()),
		Version:         uuid, // keep uuid visible in host record
		Gpu:             []string{},
	}

	// call ReportSystemInfo2 (one-shot)
	callCtx, cancel := context.WithTimeout(context.Background(), 6*time.Second)
	_, err = client.ReportSystemInfo2(callCtx, host)
	cancel()
	if err != nil {
		log.Printf("[A-%d][%s] ReportSystemInfo2 err: %v", idx, uuid, err)
		return
	}
	log.Printf("[A-%d][%s] ReportSystemInfo2 OK", idx, uuid)

	// open ReportSystemState stream and send periodic State (1s)
	stateCtx, stateCancel := context.WithCancel(context.Background())
	defer stateCancel()

	stream, err := client.ReportSystemState(stateCtx)
	if err != nil {
		log.Printf("[A-%d][%s] ReportSystemState open err: %v", idx, uuid, err)
		return
	}

	// receive goroutine to drain Receipts (server might send receipts)
	go func() {
		for {
			_, err := stream.Recv()
			if err != nil {
				// stream closed or error; exit receive loop
				return
			}
		}
	}()

	// deterministic-ish slight jitter for each agent
	r := rand.New(rand.NewSource(time.Now().UnixNano() + int64(idx)))

	for {
		state := &pb.State{
			Cpu:            r.Float64() * 10, // small values to reduce server load
			MemUsed:        512 * 1024 * 1024,
			SwapUsed:       0,
			DiskUsed:       10 * 1024 * 1024 * 1024,
			NetInTransfer:  0,
			NetOutTransfer: 0,
			NetInSpeed:     0,
			NetOutSpeed:    0,
			Uptime:         uint64(time.Now().Unix()),
			Load1:          r.Float64(),
			Load5:          r.Float64(),
			Load15:         r.Float64(),
			TcpConnCount:   1,
			UdpConnCount:   0,
			ProcessCount:   20,
			Gpu:            []float64{},
		}

		if err := stream.Send(state); err != nil {
			log.Printf("[A-%d][%s] state send err: %v", idx, uuid, err)
			return
		}
		time.Sleep(1 * time.Second)
	}
}

func main() {
	server := flag.String("server", "", "nezha server address host:port (required)")
	secret := flag.String("secret", "", "client secret / clientSecret (required)")
	count := flag.Int("count", 1, "number of agents to simulate")
	tlsFlag := flag.Bool("tls", false, "use TLS for gRPC (default false)")
	flag.Parse()

	if *server == "" || *secret == "" {
		log.Fatal("--server and --secret are required")
	}

	log.Printf("Starting %d agents -> %s (tls=%v)\n", *count, *server, *tlsFlag)

	for i := 0; i < *count; i++ {
		go runAgent(*server, *secret, i, *tlsFlag)
	}

	select {}
}
