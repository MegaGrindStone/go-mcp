package main

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/MegaGrindStone/go-mcp/pkg/mcp"
	"github.com/google/uuid"
)

type client struct {
	cli    mcp.SSEClient
	ctx    context.Context
	cancel context.CancelFunc

	notifications []string
	logs          []string

	closeLock sync.Mutex
	closed    bool
	done      chan struct{}
}

const exitCommand = "exit"

func newClient() *client {
	ctx, cancel := context.WithCancel(context.Background())
	c := client{
		ctx:    ctx,
		cancel: cancel,
		done:   make(chan struct{}),
	}

	url := fmt.Sprintf("%s/sse", baseURL())
	c.cli = mcp.NewSSEClient(&c, url, http.DefaultClient,
		mcp.WithClientPingInterval(30*time.Second),
		mcp.WithSamplingHandler(&c),
		mcp.WithResourceSubscribedWatcher(&c),
		mcp.WithProgressListener(&c),
		mcp.WithLogReceiver(&c),
	)

	return &c
}

func (c *client) Info() mcp.Info {
	return mcp.Info{
		Name:    "everything-client",
		Version: "1.0",
	}
}

func (c *client) RequirePromptServer() bool {
	return true
}

func (c *client) RequireResourceServer() bool {
	return true
}

func (c *client) RequireToolServer() bool {
	return true
}

func (c *client) CreateSampleMessage(_ context.Context, params mcp.SamplingParams) (mcp.SamplingResult, error) {
	userPrompt := params.Messages[0].Content.Text
	return mcp.SamplingResult{
		Role: mcp.PromptRoleAssistant,
		Content: mcp.SamplingContent{
			Type: mcp.ContentTypeText,
			Text: fmt.Sprintf("This is a sample message from external LLM for prompt \"%s\" with max tokens %d",
				userPrompt, params.MaxTokens),
		},
		Model:      "ai-overlord-1.0",
		StopReason: "finished",
	}, nil
}

func (c *client) OnResourceSubscribedChanged(uri string) {
	notif := fmt.Sprintf("Update for resource %s received at %s", uri, time.Now().Format(time.RFC3339))
	c.notifications = append(c.notifications, notif)
}

func (c *client) OnProgress(params mcp.ProgressParams) {
	fmt.Printf("Progress: %f/%f\n", params.Progress, params.Total)
}

func (c *client) OnLog(params mcp.LogParams) {
	l := fmt.Sprintf("%s: Level %d: %s", time.Now().Format(time.RFC3339), params.Level, params.Data.Message)
	c.logs = append(c.logs, l)
}

func (c *client) run() {
	defer c.stop()
	go c.listenInterruptSignal()

	fmt.Println("Connecting to server...")
	sessID, err := c.cli.Connect(c.ctx)
	if err != nil {
		fmt.Printf("failed to connect to server: %v\n", err)
		return
	}
	fmt.Printf("Connected to server with session ID %s\n", sessID)

	for {
		fmt.Println()
		fmt.Println("1. Prompts")
		fmt.Println("2. Resources")
		fmt.Println("3. Tools")
		fmt.Println("4. Notifications")
		fmt.Println("5. Server Log")
		fmt.Println("6. Exit")

		fmt.Println()
		fmt.Print("Enter command number: ")

		input, err := c.waitStdIOInput()
		if err != nil {
			if errors.Is(err, os.ErrClosed) {
				return
			}
			fmt.Print(err)
			continue
		}

		exit := false
		switch input {
		case "1":
			exit = c.runPrompts(sessID)
		case "2":
			exit = c.runResources(sessID)
		case "3":
			exit = c.runTools(sessID)
		case "4":
			c.runNotifications()
		case "5":
			exit = c.runLogs()
		case "6":
			return
		default:
			fmt.Println("Invalid command")
		}

		if exit {
			return
		}
	}
}

func (c *client) runPrompts(sessID string) bool {
	listPrompts, err := c.cli.ListPrompts(c.ctx, sessID, mcp.PromptsListParams{})
	if err != nil {
		log.Printf("failed to list prompts: %v", err)
		return true
	}

	fmt.Println()
	fmt.Println("List Prompts")
	fmt.Println()

	for _, prompt := range listPrompts.Prompts {
		fmt.Printf("Prompt: %s\n", prompt.Name)
	}

	fmt.Println()
	fmt.Print("Enter prompt name (type exit to go back):")

	input, err := c.waitStdIOInput()
	if err != nil {
		if errors.Is(err, os.ErrClosed) {
			return true
		}
		fmt.Print(err)
		return false
	}

	if input == exitCommand {
		return false
	}

	promptIdx := slices.IndexFunc(listPrompts.Prompts, func(p mcp.Prompt) bool {
		return p.Name == input
	})
	if promptIdx == -1 {
		fmt.Printf("Invalid prompt name: %s\n", input)
		return false
	}
	prompt := listPrompts.Prompts[promptIdx]

	params := mcp.PromptsGetParams{
		Name: prompt.Name,
	}
	if prompt.Name == "complex-prompt" {
		var exit bool
		params, exit = c.runComplexPrompt(sessID)
		if exit {
			return true
		}
	}

	pr, err := c.cli.GetPrompt(c.ctx, sessID, params)
	if err != nil {
		fmt.Printf("Failed to get prompt: %v\n", err)
		return false
	}

	fmt.Println()
	fmt.Println("Prompt Messages:")
	for _, msg := range pr.Messages {
		fmt.Println("---")
		fmt.Printf("Role: %s\n", msg.Role)
		switch msg.Content.Type {
		case mcp.ContentTypeText:
			fmt.Printf("Message: %s\n", msg.Content.Text)
		case mcp.ContentTypeImage:
			// Truncate the image data, as the terminal can't display it anyway.
			data := msg.Content.Data[0:50]
			fmt.Printf("Truncated image data: %s...\n", data)
		case mcp.ContentTypeResource:
			fmt.Printf("Message: Resource\n")
		}
		fmt.Println("---")
	}

	fmt.Println("Press enter to continue...")

	_, err = c.waitStdIOInput()
	if err != nil {
		if errors.Is(err, os.ErrClosed) {
			return true
		}
		fmt.Print(err)
		return false
	}

	return false
}

func (c *client) runComplexPrompt(sessID string) (mcp.PromptsGetParams, bool) {
	fmt.Println(`
Pardon the implementation of 'autocomplete' in this example, but it's a good idea to implement it in your own client.`)

	temperature, exit := c.runPromptAutocomplete(sessID, "temperature")
	if exit {
		return mcp.PromptsGetParams{}, true
	}
	style, exit := c.runPromptAutocomplete(sessID, "style")
	if exit {
		return mcp.PromptsGetParams{}, true
	}

	fmt.Printf("Temperature: %s\n", temperature)
	fmt.Printf("Style: %s\n", style)

	return mcp.PromptsGetParams{
		Name:      "complex-prompt",
		Arguments: map[string]string{"temperature": temperature, "style": style},
	}, false
}

func (c *client) runPromptAutocomplete(sessID string, field string) (string, bool) {
	for {
		fmt.Printf("Insert a %s:", field)

		input, err := c.waitStdIOInput()
		if err != nil {
			if errors.Is(err, os.ErrClosed) {
				return "", true
			}
			fmt.Print(err)
			continue
		}

		if input == exitCommand {
			return "", true
		}

		ac, err := c.cli.CompletesPrompt(c.ctx, sessID, mcp.CompletionCompleteParams{
			Ref: mcp.CompletionCompleteRef{
				Type: mcp.CompletionRefPrompt,
				Name: "complex-prompt",
			},
			Argument: mcp.CompletionArgument{
				Name:  field,
				Value: input,
			},
		})
		if err != nil {
			fmt.Printf("Failed to get autocomplete: %v\n", err)
			continue
		}

		if len(ac.Completion.Values) == 0 {
			fmt.Println(`
Your input is not found in the list of possible completions, input an empty string to list all possible completions.`)
			continue
		}

		idx := slices.IndexFunc(ac.Completion.Values, func(v string) bool {
			return v == input
		})
		if idx > -1 {
			return ac.Completion.Values[idx], false
		}

		fmt.Println()
		fmt.Println("Autocomplete:")
		for _, c := range ac.Completion.Values {
			fmt.Printf("%s\n", c)
		}
		fmt.Println()
	}
}

//nolint:funlen
func (c *client) runResources(sessID string) bool {
	cursor := ""
	for {
		listResources, err := c.cli.ListResources(c.ctx, sessID, mcp.ResourcesListParams{
			Cursor: cursor,
		})
		if err != nil {
			log.Printf("failed to list resources: %v", err)
			return true
		}

		fmt.Println()
		fmt.Println("List Resources")
		fmt.Println()

		for _, resource := range listResources.Resources {
			fmt.Printf("Resource URI: %s\n", resource.URI)
		}

		fmt.Println()
		fmt.Println("Enter one of the following commands:")
		fmt.Println("- read <resourceURI>: Read the content of the resource")
		fmt.Println("- subscribe <resourceURI>: Subscribe to updates for the resource (nop if already subscribed)")
		fmt.Println("- unsubscribe <resourceURI>: Unsubscribe from updates for the resource (nop if not subscribed)")
		fmt.Println("- next: Go to the next page (return to first page if already at the last page)")
		fmt.Println("- exit: Go back to the main menu")

		input, err := c.waitStdIOInput()
		if err != nil {
			if errors.Is(err, os.ErrClosed) {
				return true
			}
			fmt.Print(err)
			return false
		}

		if input == exitCommand {
			return false
		}
		if input == "next" {
			cursor = listResources.NextCursor
			continue
		}

		inputArr := strings.Split(input, " ")
		if len(inputArr) < 2 {
			fmt.Printf("Invalid command: %s\n", input)
			return false
		}

		resourceIdx := slices.IndexFunc(listResources.Resources, func(r mcp.Resource) bool {
			return r.URI == inputArr[1]
		})

		if resourceIdx == -1 {
			fmt.Printf("Invalid resourceURI: %s\n", input)
			return false
		}
		resource := listResources.Resources[resourceIdx]

		if inputArr[0] == "subscribe" {
			if err := c.cli.SubscribeResource(c.ctx, sessID, mcp.ResourcesSubscribeParams{
				URI: resource.URI,
			}); err != nil {
				fmt.Printf("Failed to subscribe to resource: %v\n", err)
			}
			fmt.Printf("Subscribed to resource %s, check Notifications for updates\n", resource.URI)
			return false
		}
		if inputArr[0] == "unsubscribe" {
			if err := c.cli.UnsubscribeResource(c.ctx, sessID, mcp.ResourcesSubscribeParams{
				URI: resource.URI,
			}); err != nil {
				fmt.Printf("Failed to unsubscribe from resource: %v\n", err)
			}
			fmt.Printf("Unsubscribed from resource %s\n", resource.URI)
			return false
		}

		rs, err := c.cli.ReadResource(c.ctx, sessID, mcp.ResourcesReadParams{
			URI: resource.URI,
		})
		if err != nil {
			fmt.Printf("Failed to get resource: %v\n", err)
			return false
		}

		fmt.Println()
		fmt.Printf("Data for resource %s:\n", resource.URI)
		switch rs.MimeType {
		case "text/plain":
			fmt.Println(rs.Text)
		case "application/octet-stream":
			fmt.Printf("Binary data of length %d\n", len(rs.Blob))
		default:
			fmt.Printf("Unknown data type: %s\n", rs.MimeType)
		}

		fmt.Println("Press enter to continue...")

		_, err = c.waitStdIOInput()
		if err != nil {
			if errors.Is(err, os.ErrClosed) {
				return true
			}
			fmt.Print(err)
			return false
		}

		break
	}

	return false
}

//nolint:funlen
func (c *client) runTools(sessID string) bool {
	listTools, err := c.cli.ListTools(c.ctx, sessID, mcp.ToolsListParams{})
	if err != nil {
		log.Printf("failed to list tools: %v", err)
		return true
	}

	fmt.Println()
	fmt.Println("List Tools")
	fmt.Println()

	for _, tool := range listTools.Tools {
		fmt.Printf("Tool: %s\n", tool.Name)
	}

	fmt.Println()
	fmt.Print("Enter tool name to call (type exit to go back):")

	input, err := c.waitStdIOInput()
	if err != nil {
		if errors.Is(err, os.ErrClosed) {
			return true
		}
		fmt.Print(err)
		return false
	}

	if input == exitCommand {
		return false
	}

	toolIdx := slices.IndexFunc(listTools.Tools, func(t mcp.Tool) bool {
		return t.Name == input
	})
	if toolIdx == -1 {
		fmt.Printf("Invalid tool name: %s\n", input)
		return false
	}
	tool := listTools.Tools[toolIdx]

	params := mcp.ToolsCallParams{
		Name: tool.Name,
	}
	var exit bool
	switch tool.Name {
	case "echo":
		params, exit = c.toolEchoParams()
		if exit {
			return true
		}
	case "add":
		params, exit = c.toolAddParams()
		if exit {
			return true
		}
	case "longRunningOperation":
		params, exit = c.toolLongRunningOperationParams()
		if exit {
			return true
		}
	case "sampleLLM":
		params, exit = c.toolSampleLLMParams()
		if exit {
			return true
		}
	case "printEnv", "getTinyImage":
	}

	tr, err := c.cli.CallTool(c.ctx, sessID, params)
	if err != nil {
		fmt.Printf("Failed to call tool: %v\n", err)
		return false
	}

	fmt.Println()
	fmt.Println("Tool Results:")
	for _, msg := range tr.Content {
		fmt.Println("---")
		switch msg.Type {
		case mcp.ContentTypeText:
			fmt.Printf("Message: %s\n", msg.Text)
		case mcp.ContentTypeImage:
			// Truncate the image data, as the terminal can't display it anyway.
			data := msg.Data[0:50]
			fmt.Printf("Truncated image data: %s...\n", data)
		case mcp.ContentTypeResource:
			fmt.Printf("Message: Resource\n")
		}
		fmt.Println("---")
	}

	fmt.Println("Press enter to continue...")

	_, err = c.waitStdIOInput()
	if err != nil {
		if errors.Is(err, os.ErrClosed) {
			return true
		}
		fmt.Print(err)
		return false
	}

	return false
}

func (c *client) toolEchoParams() (mcp.ToolsCallParams, bool) {
	for {
		fmt.Println("Enter the message to echo:")

		input, err := c.waitStdIOInput()
		if err != nil {
			if errors.Is(err, os.ErrClosed) {
				return mcp.ToolsCallParams{}, true
			}
			fmt.Print(err)
			continue
		}

		return mcp.ToolsCallParams{
			Name: "echo",
			Arguments: map[string]any{
				"message": input,
			},
		}, false
	}
}

func (c *client) toolAddParams() (mcp.ToolsCallParams, bool) {
	for {
		fmt.Println("Enter two numbers to add (separated by space):")

		input, err := c.waitStdIOInput()
		if err != nil {
			if errors.Is(err, os.ErrClosed) {
				return mcp.ToolsCallParams{}, true
			}
			fmt.Print(err)
			continue
		}

		inputArr := strings.Split(input, " ")
		if len(inputArr) != 2 {
			fmt.Printf("Invalid input: %s\n", input)
			continue
		}

		a, err := strconv.ParseFloat(inputArr[0], 64)
		if err != nil {
			fmt.Printf("Invalid input: %s\n", input)
			continue
		}
		b, err := strconv.ParseFloat(inputArr[1], 64)
		if err != nil {
			fmt.Printf("Invalid input: %s\n", input)
			continue
		}

		return mcp.ToolsCallParams{
			Name: "add",
			Arguments: map[string]any{
				"a": a,
				"b": b,
			},
		}, false
	}
}

func (c *client) toolLongRunningOperationParams() (mcp.ToolsCallParams, bool) {
	for {
		fmt.Println("Enter duration and steps (separated by space):")

		input, err := c.waitStdIOInput()
		if err != nil {
			if errors.Is(err, os.ErrClosed) {
				return mcp.ToolsCallParams{}, true
			}
			fmt.Print(err)
			continue
		}

		inputArr := strings.Split(input, " ")
		if len(inputArr) != 2 {
			fmt.Printf("Invalid input: %s\n", input)
			continue
		}

		duration, err := strconv.ParseFloat(inputArr[0], 64)
		if err != nil {
			fmt.Printf("Invalid input: %s\n", input)
			continue
		}

		steps, err := strconv.ParseFloat(inputArr[1], 64)
		if err != nil {
			fmt.Printf("Invalid input: %s\n", input)
			continue
		}

		return mcp.ToolsCallParams{
			Name: "longRunningOperation",
			Arguments: map[string]any{
				"duration": duration,
				"steps":    steps,
			},
			Meta: mcp.ParamsMeta{
				ProgressToken: mcp.MustString(uuid.New().String()),
			},
		}, false
	}
}

func (c *client) toolSampleLLMParams() (mcp.ToolsCallParams, bool) {
	for {
		fmt.Println("Enter the prompt:")

		input, err := c.waitStdIOInput()
		if err != nil {
			if errors.Is(err, os.ErrClosed) {
				return mcp.ToolsCallParams{}, true
			}
			fmt.Print(err)
			continue
		}

		prompt := input

		fmt.Println("Enter the max tokens:")

		input, err = c.waitStdIOInput()
		if err != nil {
			if errors.Is(err, os.ErrClosed) {
				return mcp.ToolsCallParams{}, true
			}
			fmt.Print(err)
			continue
		}

		maxTokens, err := strconv.ParseFloat(input, 64)
		if err != nil {
			fmt.Printf("Invalid input: %s\n", input)
			continue
		}

		return mcp.ToolsCallParams{
			Name: "sampleLLM",
			Arguments: map[string]any{
				"prompt":    prompt,
				"maxTokens": maxTokens,
			},
		}, false
	}
}

func (c *client) runNotifications() {
	fmt.Println()

	if len(c.notifications) == 0 {
		fmt.Println("No notifications received")
		return
	}

	fmt.Println("List Notifications")
	fmt.Println()

	for _, n := range c.notifications {
		fmt.Printf("Notification: %s\n", n)
	}
}

func (c *client) runLogs() bool {
	for {
		fmt.Println()

		if len(c.logs) == 0 {
			fmt.Println("No logs received")
		} else {
			fmt.Println("List Logs")
			fmt.Println()

			for _, l := range c.logs {
				fmt.Printf("%s\n", l)
			}
		}

		logLevels := []string{"debug", "info", "notice", "warning", "error", "critical", "alert", "emergency"}

		fmt.Println()
		fmt.Printf("Enter log level to set, available log levels: %s (type exit to go back):\n",
			strings.Join(logLevels, ", "))

		input, err := c.waitStdIOInput()
		if err != nil {
			if errors.Is(err, os.ErrClosed) {
				return true
			}
			fmt.Print(err)
			continue
		}

		if input == exitCommand {
			return false
		}

		var level mcp.LogLevel
		switch input {
		case "debug":
			level = mcp.LogLevelDebug
		case "info":
			level = mcp.LogLevelInfo
		case "notice":
			level = mcp.LogLevelNotice
		case "warning":
			level = mcp.LogLevelWarning
		case "error":
			level = mcp.LogLevelError
		case "critical":
			level = mcp.LogLevelCritical
		case "alert":
			level = mcp.LogLevelAlert
		case "emergency":
			level = mcp.LogLevelEmergency
		}

		if err := c.cli.SetLogLevel(level); err != nil {
			fmt.Printf("Failed to set log level: %v\n", err)
			continue
		}

		fmt.Println("Log level set to", input)
		break
	}
	return false
}

func (c *client) listenInterruptSignal() {
	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, os.Interrupt)
	<-signalChan
	c.stop()
}

func (c *client) waitStdIOInput() (string, error) {
	inputChan := make(chan string)
	errsChan := make(chan error)
	go func() {
		scanner := bufio.NewScanner(os.Stdin)
		if scanner.Scan() {
			inputChan <- scanner.Text()
		}
		if err := scanner.Err(); err != nil {
			errsChan <- err
		}
	}()

	select {
	case <-c.ctx.Done():
		return "", os.ErrClosed
	case <-c.done:
		return "", os.ErrClosed
	case err := <-errsChan:
		return "", err
	case input := <-inputChan:
		return input, nil
	}
}

func (c *client) stop() {
	c.closeLock.Lock()
	defer c.closeLock.Unlock()

	c.cancel()
	if !c.closed {
		close(c.done)
		c.closed = true
	}
}