package mcp_test

// func TestStdIOBidirectionalMessageFlow(t *testing.T) {
// 	// Create buffered pipes to simulate stdin/stdout
// 	clientReader, serverWriter := io.Pipe()
// 	serverReader, clientWriter := io.Pipe()
//
// 	// Create StdIO instances
// 	serverTransport := mcp.NewStdIO(serverReader, serverWriter)
// 	clientTransport := mcp.NewStdIO(clientReader, clientWriter)
//
// 	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
// 	defer cancel()
//
// 	// Prepare test messages
// 	testMessages := []mcp.JSONRPCMessage{
// 		{
// 			JSONRPC: mcp.JSONRPCVersion,
// 			Method:  "request1",
// 			Params:  json.RawMessage(`{"data": "first request"}`),
// 		},
// 		{
// 			JSONRPC: mcp.JSONRPCVersion,
// 			Method:  "request2",
// 			Params:  json.RawMessage(`{"data": "second request"}`),
// 		},
// 	}
//
// 	// Channels to track message exchanges
// 	clientReceivedMsgs := make([]mcp.JSONRPCMessage, 0)
// 	serverReceivedMsgs := make([]mcp.JSONRPCMessage, 0)
//
// 	// Prepare client session
// 	ready := make(chan error, 1)
// 	clientMsgs, err := clientTransport.StartSession(ctx, ready)
// 	if err != nil {
// 		t.Fatalf("failed to start client session: %v", err)
// 	}
//
// 	// Wait for connection readiness
// 	if err := <-ready; err != nil {
// 		t.Fatalf("connection not ready: %v", err)
// 	}
//
// 	// Get server session
// 	var serverSession mcp.Session
// 	sessions := make(chan mcp.Session, 1)
// 	go func() {
// 		for s := range serverTransport.Sessions() {
// 			sessions <- s
// 		}
// 	}()
// 	serverSession = <-sessions
// 	defer serverSession.Stop()
//
// 	// Synchronization for message tracking
// 	var wg sync.WaitGroup
// 	wg.Add(2)
//
// 	// Receive messages on client side
// 	go func() {
// 		defer wg.Done()
// 		for msg := range clientMsgs {
// 			clientReceivedMsgs = append(clientReceivedMsgs, msg)
// 			if len(clientReceivedMsgs) == len(testMessages) {
// 				return
// 			}
// 		}
// 	}()
//
// 	// Receive messages on server side
// 	go func() {
// 		defer wg.Done()
// 		for msg := range serverSession.Messages() {
// 			serverReceivedMsgs = append(serverReceivedMsgs, msg)
// 			if len(serverReceivedMsgs) == len(testMessages) {
// 				return
// 			}
// 		}
// 	}()
//
// 	// Send messages in both directions
// 	for _, msg := range testMessages {
// 		// Server to client
// 		if err := serverSession.Send(msg); err != nil {
// 			t.Fatalf("failed to send server message: %v", err)
// 		}
//
// 		// Client to server
// 		clientResponseMsg := mcp.JSONRPCMessage{
// 			JSONRPC: mcp.JSONRPCVersion,
// 			Method:  "response_" + msg.Method,
// 			Params:  json.RawMessage(`{"received": "` + msg.Method + `"}`),
// 		}
// 		if err := clientTransport.Send(ctx, clientResponseMsg); err != nil {
// 			t.Fatalf("failed to send client message: %v", err)
// 		}
// 	}
//
// 	// Wait for message collection
// 	wg.Wait()
//
// 	// Verify message flow
// 	if len(clientReceivedMsgs) != len(testMessages) {
// 		t.Errorf("client did not receive all messages. Got %d, want %d",
// 			len(clientReceivedMsgs), len(testMessages))
// 	}
//
// 	if len(serverReceivedMsgs) != len(testMessages) {
// 		t.Errorf("server did not receive all messages. Got %d, want %d",
// 			len(serverReceivedMsgs), len(testMessages))
// 	}
//
// 	for i, msg := range testMessages {
// 		if clientReceivedMsgs[i].Method != msg.Method {
// 			t.Errorf("client received wrong message. Got %s, want %s",
// 				clientReceivedMsgs[i].Method, msg.Method)
// 		}
//
// 		if serverReceivedMsgs[i].Method != "response_"+msg.Method {
// 			t.Errorf("server received wrong response. Got %s, want response_%s",
// 				serverReceivedMsgs[i].Method, msg.Method)
// 		}
// 	}
// }
//
// func TestStdIOLargeMessagePayload(t *testing.T) {
// 	// Create buffered pipes to simulate stdin/stdout
// 	clientReader, serverWriter := io.Pipe()
// 	serverReader, clientWriter := io.Pipe()
//
// 	// Create StdIO instances
// 	serverTransport := mcp.NewStdIO(serverReader, serverWriter)
// 	clientTransport := mcp.NewStdIO(clientReader, clientWriter)
//
// 	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
// 	defer cancel()
//
// 	// Get server session
// 	var serverSession mcp.Session
// 	sessions := make(chan mcp.Session, 1)
// 	go func() {
// 		for s := range serverTransport.Sessions() {
// 			sessions <- s
// 		}
// 	}()
// 	serverSession = <-sessions
// 	go func(sess mcp.Session) {
// 		for msg := range sess.Messages() {
// 			t.Logf("received message: %s", msg.Method)
// 		}
// 	}(serverSession)
// 	defer serverSession.Stop()
//
// 	// Payload sizes to test
// 	payloadSizes := []int{
// 		1 * 1024,        // 1 KB
// 		100 * 1024,      // 100 KB
// 		1 * 1024 * 1024, // 1 MB
// 	}
//
// 	for _, size := range payloadSizes {
// 		t.Run(fmt.Sprintf("PayloadSize_%d", size), func(t *testing.T) {
// 			// Generate random JSON payload
// 			// This payload is required to be JSON message, instead of fully random bytes, because we want to test the
// 			// handling of the message payload in the server, not failing on unmarshalling the JSON.
// 			payload := generateRandomJSON(size)
//
// 			// Create a large message
// 			largeMsg := mcp.JSONRPCMessage{
// 				JSONRPC: mcp.JSONRPCVersion,
// 				Method:  "largePayload",
// 				Params:  payload,
// 			}
//
// 			// Prepare client session
// 			ready := make(chan error, 1)
// 			clientMsgs, err := clientTransport.StartSession(ctx, ready)
// 			if err != nil {
// 				t.Fatalf("failed to start client session: %v", err)
// 			}
//
// 			// Wait for connection readiness
// 			if err := <-ready; err != nil {
// 				t.Fatalf("connection not ready: %v", err)
// 			}
//
// 			// Channel to track message receipt
// 			receivedChan := make(chan mcp.JSONRPCMessage, 1)
//
// 			// Goroutine to receive message
// 			go func() {
// 				for msg := range clientMsgs {
// 					receivedChan <- msg
// 					break
// 				}
// 			}()
//
// 			// Send large message from server to client
// 			if err := serverSession.Send(largeMsg); err != nil {
// 				t.Fatalf("failed to send large message: %v", err)
// 			}
//
// 			// Wait for message receipt
// 			select {
// 			case receivedMsg := <-receivedChan:
// 				// Verify message method
// 				if receivedMsg.Method != largeMsg.Method {
// 					t.Errorf("Incorrect method received. Got %s, want %s",
// 						receivedMsg.Method, largeMsg.Method)
// 				}
//
// 			case <-time.After(5 * time.Second):
// 				t.Fatalf("Timeout waiting for large message of size %d", size)
// 			}
// 		})
// 	}
// }
//
// func TestStdIOMalformedJSONHandling(t *testing.T) {
// 	// Create buffered pipes to simulate stdin/stdout
// 	clientReader, serverWriter := io.Pipe()
// 	serverReader, clientWriter := io.Pipe()
//
// 	// Create StdIO instances
// 	_ = mcp.NewStdIO(serverReader, serverWriter)
// 	clientTransport := mcp.NewStdIO(clientReader, clientWriter)
//
// 	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
// 	defer cancel()
//
// 	// Prepare client session
// 	ready := make(chan error, 1)
// 	clientMsgs, err := clientTransport.StartSession(ctx, ready)
// 	if err != nil {
// 		t.Fatalf("failed to start client session: %v", err)
// 	}
//
// 	// Wait for connection readiness
// 	if err := <-ready; err != nil {
// 		t.Fatalf("connection not ready: %v", err)
// 	}
//
// 	// Test scenarios for malformed JSON
// 	malformedMessages := []string{
// 		`{incomplete`,                 // Incomplete JSON
// 		`{"method": "test", invalid}`, // Invalid JSON syntax
// 		`{"method": 123}`,             // Invalid type for method
// 	}
//
// 	// Verify message handling
// 	receivedMsgs := 0
// 	go func() {
// 		for range clientMsgs {
// 			receivedMsgs++
// 		}
// 	}()
//
// 	// Send malformed messages
// 	for _, msg := range malformedMessages {
// 		// Write raw malformed message to simulate input
// 		_, err := serverWriter.Write([]byte(msg + "\n"))
// 		if err != nil {
// 			t.Fatalf("Failed to write malformed message: %v", err)
// 		}
// 	}
//
// 	// Give some time for processing
// 	time.Sleep(500 * time.Millisecond)
//
// 	// Ensure no valid messages were processed
// 	if receivedMsgs != 0 {
// 		t.Errorf("Expected 0 messages processed, got %d", receivedMsgs)
// 	}
// }
//
// func TestStdIOConcurrentMessageStress(t *testing.T) {
// 	// Create buffered pipes to simulate stdin/stdout
// 	clientReader, serverWriter := io.Pipe()
// 	serverReader, clientWriter := io.Pipe()
//
// 	// Create StdIO instances
// 	serverTransport := mcp.NewStdIO(serverReader, serverWriter)
// 	clientTransport := mcp.NewStdIO(clientReader, clientWriter)
//
// 	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
// 	defer cancel()
//
// 	// Prepare client session
// 	ready := make(chan error, 1)
// 	clientMsgs, err := clientTransport.StartSession(ctx, ready)
// 	if err != nil {
// 		t.Fatalf("failed to start client session: %v", err)
// 	}
//
// 	// Wait for connection readiness
// 	if err := <-ready; err != nil {
// 		t.Fatalf("connection not ready: %v", err)
// 	}
//
// 	// Get server session
// 	var serverSession mcp.Session
// 	for s := range serverTransport.Sessions() {
// 		serverSession = s
// 		break
// 	}
//
// 	// Number of concurrent messages
// 	messageCount := 1000
//
// 	// Synchronization primitives
// 	var clientReceived, serverReceived sync.WaitGroup
// 	clientReceived.Add(messageCount)
// 	serverReceived.Add(messageCount)
//
// 	// Track any errors during concurrent sending
// 	var errorsFound sync.Map
//
// 	// Goroutine for client message receiving
// 	go func() {
// 		for msg := range clientMsgs {
// 			// Verify message content
// 			if msg.Method != "concurrentTest" {
// 				errorsFound.Store("client_invalid_method", fmt.Errorf("unexpected method: %s", msg.Method))
// 			}
// 			clientReceived.Done()
// 		}
// 	}()
//
// 	// Goroutine for server message receiving
// 	go func() {
// 		for msg := range serverSession.Messages() {
// 			// Verify message content
// 			if msg.Method != "concurrentResponse" {
// 				errorsFound.Store("server_invalid_method", fmt.Errorf("unexpected method: %s", msg.Method))
// 			}
// 			serverReceived.Done()
// 		}
// 	}()
//
// 	// Concurrent message sending
// 	var sendWg sync.WaitGroup
// 	sendWg.Add(2)
//
// 	go func() {
// 		defer sendWg.Done()
// 		for i := 0; i < messageCount; i++ {
// 			msg := mcp.JSONRPCMessage{
// 				JSONRPC: mcp.JSONRPCVersion,
// 				Method:  "concurrentTest",
// 				Params:  json.RawMessage(fmt.Sprintf(`{"index": %d}`, i)),
// 			}
// 			if err := serverSession.Send(msg); err != nil {
// 				errorsFound.Store(fmt.Sprintf("server_send_%d", i), err)
// 				break
// 			}
// 		}
// 	}()
//
// 	go func() {
// 		defer sendWg.Done()
// 		for i := 0; i < messageCount; i++ {
// 			msg := mcp.JSONRPCMessage{
// 				JSONRPC: mcp.JSONRPCVersion,
// 				Method:  "concurrentResponse",
// 				Params:  json.RawMessage(fmt.Sprintf(`{"index": %d}`, i)),
// 			}
// 			if err := clientTransport.Send(ctx, msg); err != nil {
// 				errorsFound.Store(fmt.Sprintf("client_send_%d", i), err)
// 				break
// 			}
// 		}
// 	}()
//
// 	// Wait for all sending to complete
// 	sendWg.Wait()
//
// 	// Wait for message processing
// 	clientReceivedDone := make(chan struct{})
// 	serverReceivedDone := make(chan struct{})
//
// 	go func() {
// 		clientReceived.Wait()
// 		close(clientReceivedDone)
// 	}()
//
// 	go func() {
// 		serverReceived.Wait()
// 		close(serverReceivedDone)
// 	}()
//
// 	// Wait for all messages or timeout
// 	select {
// 	case <-clientReceivedDone:
// 	case <-ctx.Done():
// 		t.Fatal("Timeout waiting for client messages")
// 	}
//
// 	select {
// 	case <-serverReceivedDone:
// 	case <-ctx.Done():
// 		t.Fatal("Timeout waiting for server messages")
// 	}
//
// 	// Check for any errors during concurrent processing
// 	errorsFound.Range(func(key, value interface{}) bool {
// 		t.Errorf("Error during concurrent test: %s - %v", key, value)
// 		return true
// 	})
// }
