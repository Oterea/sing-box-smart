package grpcactivity

import (
	"context"
	"net"
	"sing-box-smart/internal/activity"
	daemon "sing-box-smart/internal/singboxapi/daemon"
	"testing"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/test/bufconn"
)

type activityServer struct {
	daemon.UnimplementedStartedServiceServer
}

func (activityServer) SubscribeConnections(_ *daemon.SubscribeConnectionsRequest, stream daemon.StartedService_SubscribeConnectionsServer) error {
	return stream.Send(&daemon.ConnectionEvents{Events: []*daemon.ConnectionEvent{{
		Type:       daemon.ConnectionEventType_CONNECTION_EVENT_NEW,
		Id:         "user-1",
		Connection: &daemon.Connection{Domain: "www.example.com", Inbound: "tun-in", InboundType: "tun"},
	}}})
}

func TestConsumeMapsConnectionEvents(t *testing.T) {
	listener := bufconn.Listen(1024 * 1024)
	server := grpc.NewServer()
	daemon.RegisterStartedServiceServer(server, activityServer{})
	go func() { _ = server.Serve(listener) }()
	defer server.Stop()

	conn, err := grpc.DialContext(context.Background(), "bufnet", grpc.WithInsecure(), grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) { return listener.Dial() }))
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()

	source := New(Config{ProbeDomains: []string{"generate.example"}})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	sink := make(chan activity.Event, 1)
	go func() { _ = source.consume(ctx, conn, sink) }()
	select {
	case event := <-sink:
		if event.Kind != activity.ConnectionNew || event.ID != "user-1" || event.Probe {
			t.Fatalf("unexpected activity event: %+v", event)
		}
	case <-time.After(time.Second):
		t.Fatal("did not receive gRPC connection event")
	}
}
