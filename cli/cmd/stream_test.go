package cmd

import (
	"context"
	"errors"
	"io"
	"testing"

	"github.com/alpemreelmas/kaptan/cli/client"
	agentv1 "github.com/alpemreelmas/kaptan/proto/agent/v1"
	"google.golang.org/grpc/metadata"
)

// mockDeployStream implements grpc.ServerStreamingClient[agentv1.ExecEvent].
type mockDeployStream struct {
	events []*agentv1.ExecEvent
	idx    int
	err    error // returned after events are exhausted (nil → EOF)
}

func (m *mockDeployStream) Recv() (*agentv1.ExecEvent, error) {
	if m.idx >= len(m.events) {
		if m.err != nil {
			return nil, m.err
		}
		return nil, io.EOF
	}
	ev := m.events[m.idx]
	m.idx++
	return ev, nil
}

func (m *mockDeployStream) Header() (metadata.MD, error) { return nil, nil }
func (m *mockDeployStream) Trailer() metadata.MD         { return nil }
func (m *mockDeployStream) CloseSend() error             { return nil }
func (m *mockDeployStream) Context() context.Context     { return context.Background() }
func (m *mockDeployStream) SendMsg(interface{}) error    { return nil }
func (m *mockDeployStream) RecvMsg(interface{}) error    { return nil }

// --- streamToStdout tests ---

func TestStreamToStdout_Success(t *testing.T) {
	stream := &mockDeployStream{
		events: []*agentv1.ExecEvent{
			{Line: "step 1"},
			{Line: "step 2"},
			{Done: true, ExitCode: 0},
		},
	}
	if err := streamToStdout("test-server", stream); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestStreamToStdout_NonZeroExit(t *testing.T) {
	stream := &mockDeployStream{
		events: []*agentv1.ExecEvent{
			{Line: "oops", IsStderr: true},
			{Done: true, ExitCode: 1},
		},
	}
	err := streamToStdout("test-server", stream)
	if err == nil {
		t.Fatal("expected error for non-zero exit code")
	}
}

func TestStreamToStdout_RecvError(t *testing.T) {
	stream := &mockDeployStream{
		events: []*agentv1.ExecEvent{
			{Line: "partial output"},
		},
		err: errors.New("connection reset"),
	}
	err := streamToStdout("test-server", stream)
	if err == nil {
		t.Fatal("expected error from Recv failure")
	}
}

// --- deployToTag validation tests ---

func TestDeployToTag_NoTagError(t *testing.T) {
	deployTag = ""
	err := deployToTag(&client.GlobalConfig{})
	if err == nil {
		t.Fatal("expected error when --tag is empty")
	}
}

func TestDeployToTag_NoMatchingServers(t *testing.T) {
	deployTag = "prod"
	global := &client.GlobalConfig{
		Servers: []client.ServerEntry{
			{Name: "web-1", Tags: []string{"staging"}},
		},
	}
	err := deployToTag(global)
	if err == nil {
		t.Fatal("expected error when no servers match tag")
	}
}
