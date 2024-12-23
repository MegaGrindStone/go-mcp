package mcp_test

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/MegaGrindStone/go-mcp/pkg/mcp"
)

func TestStdIORun(t *testing.T) {
	type testCase struct {
		name        string
		srv         func() mcp.StdIOServer
		cli         func(srv mcp.StdIOServer) *mcp.StdIOClient
		wantSuccess bool
	}

	testCases := []testCase{
		{
			name: "success with no capabilities",
			srv: func() mcp.StdIOServer {
				return mcp.NewStdIOServer(&mockServer{})
			},
			cli: func(srv mcp.StdIOServer) *mcp.StdIOClient {
				return mcp.NewStdIOClient(&mockClient{}, srv)
			},
			wantSuccess: true,
		},
		{
			name: "success with full capabilities",
			srv: func() mcp.StdIOServer {
				return mcp.NewStdIOServer(&mockServer{
					requireRootsListClient: true,
					requireSamplingClient:  true,
				}, mcp.WithPromptServer(mockPromptServer{}),
					mcp.WithResourceServer(mockResourceServer{}),
					mcp.WithToolServer(mockToolServer{}),
					mcp.WithLogHandler(mockLogHandler{}),
					mcp.WithRootsListWatcher(mockRootsListWatcher{}),
				)
			},
			cli: func(srv mcp.StdIOServer) *mcp.StdIOClient {
				return mcp.NewStdIOClient(&mockClient{
					requirePromptServer:   true,
					requireResourceServer: true,
					requireToolServer:     true,
				}, srv, mcp.WithRootsListHandler(mockRootsListHandler{}),
					mcp.WithRootsListUpdater(mockRootsListUpdater{}),
					mcp.WithSamplingHandler(mockSamplingHandler{}),
					mcp.WithLogReceiver(mockLogReceiver{}),
				)
			},
			wantSuccess: true,
		},
		{
			name: "fail insufficient client capabilities",
			srv: func() mcp.StdIOServer {
				return mcp.NewStdIOServer(&mockServer{
					requireRootsListClient: true,
				}, mcp.WithPromptServer(mockPromptServer{}))
			},
			cli: func(srv mcp.StdIOServer) *mcp.StdIOClient {
				return mcp.NewStdIOClient(&mockClient{}, srv)
			},
			wantSuccess: false,
		},
		{
			name: "fail insufficient server capabilities",
			srv: func() mcp.StdIOServer {
				return mcp.NewStdIOServer(&mockServer{
					requireRootsListClient: true,
					requireSamplingClient:  true,
				}, mcp.WithPromptServer(mockPromptServer{}),
					mcp.WithToolServer(mockToolServer{}),
					mcp.WithLogHandler(mockLogHandler{}),
					mcp.WithRootsListWatcher(mockRootsListWatcher{}),
				)
			},
			cli: func(srv mcp.StdIOServer) *mcp.StdIOClient {
				return mcp.NewStdIOClient(&mockClient{
					requirePromptServer:   true,
					requireResourceServer: true,
					requireToolServer:     true,
				}, srv,
					mcp.WithRootsListHandler(mockRootsListHandler{}),
					mcp.WithRootsListUpdater(mockRootsListUpdater{}),
					mcp.WithSamplingHandler(mockSamplingHandler{}),
				)
			},
			wantSuccess: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			srv := tc.srv()
			cli := tc.cli(srv)

			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()

			inReader, _, _ := os.Pipe()
			_, outWriter, _ := os.Pipe()

			readyChan := make(chan struct{})
			runErrsChan := make(chan error)
			errsChan := make(chan error)

			go func() {
				errsChan <- cli.Run(ctx, inReader, outWriter, readyChan, runErrsChan)
			}()

			select {
			case <-ctx.Done():
				t.Fatalf("Run timed out")
			case <-readyChan:
			}

			var err error
			ticker := time.NewTicker(time.Second)
			select {
			case <-ctx.Done():
				t.Fatalf("Run timed out")
			case err = <-errsChan:
			case <-ticker.C:
				// No error received
			}

			if !tc.wantSuccess {
				if err == nil {
					t.Errorf("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}
		})
	}
}
