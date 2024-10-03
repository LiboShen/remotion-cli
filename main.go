package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/remotion-dev/lambda_go_sdk"
	"github.com/urfave/cli/v2"
)

func main() {
	app := &cli.App{
		Name:  "remotion-cli",
		Usage: "Trigger Remotion Lambda function to render video",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:     "serve-url",
				Aliases:  []string{"s"},
				Usage:    "URL to your Webpack bundle",
				EnvVars:  []string{"REMOTION_APP_SERVE_URL"},
				Required: true,
			},
			&cli.StringFlag{
				Name:     "function-name",
				Aliases:  []string{"f"},
				Usage:    "Lambda function name",
				EnvVars:  []string{"REMOTION_APP_FUNCTION_NAME"},
				Required: true,
			},
			&cli.StringFlag{
				Name:     "region",
				Aliases:  []string{"r"},
				Usage:    "AWS region",
				EnvVars:  []string{"REMOTION_APP_REGION"},
				Required: true,
			},
			&cli.StringFlag{
				Name:    "composition",
				Aliases: []string{"c"},
				Usage:   "Composition name",
				Value:   "main",
			},
			&cli.StringFlag{
				Name:    "input-props",
				Aliases: []string{"i"},
				Usage:   "Input props as JSON string",
				Value:   `{"data": "Let's play"}`,
			},
			&cli.BoolFlag{
				Name:    "quiet",
				Aliases: []string{"q"},
				Usage:   "Suppress output",
				Value:   false,
			},
		},
		Action: renderVideo,
	}

	err := app.Run(os.Args)
	if err != nil {
		log.Fatal(err)
	}
}

func renderVideo(c *cli.Context) error {
	renderInputRequest := lambda_go_sdk.RemotionOptions{
		ServeUrl:     c.String("serve-url"),
		FunctionName: c.String("function-name"),
		Region:       c.String("region"),
		InputProps:   map[string]interface{}{},
		Composition:  c.String("composition"),
	}

	// Parse input props
	err := json.Unmarshal([]byte(c.String("input-props")), &renderInputRequest.InputProps)
	if err != nil {
		return fmt.Errorf("failed to parse input props: %w", err)
	}

	renderResponse, renderError := lambda_go_sdk.RenderMediaOnLambda(renderInputRequest)
	if renderError != nil {
		return fmt.Errorf("render error: %w", renderError)
	}

	if !c.Bool("quiet") {
		fmt.Printf("Render ID: %s\n", renderResponse.RenderId)
		fmt.Printf("Bucket Name: %s\n", renderResponse.BucketName)
	}

	renderProgressInputRequest := lambda_go_sdk.RenderConfig{
		FunctionName: c.String("function-name"),
		Region:       c.String("region"),
		RenderId:     renderResponse.RenderId,
		BucketName:   renderResponse.BucketName,
		LogLevel:     "info",
	}

	for {
		renderProgressResponse, renderProgressError := lambda_go_sdk.GetRenderProgress(renderProgressInputRequest)
		if renderProgressError != nil {
			return fmt.Errorf("(%s) render progress error: %w", renderResponse.RenderId, renderProgressError)
		}

		if len(renderProgressResponse.Errors) > 0 {
			fmt.Println("Errors:")
			errorsJSON, _ := json.Marshal(renderProgressResponse)
			fmt.Println(string(errorsJSON))
			return fmt.Errorf("rendering failed with errors")
		}

		if !c.Bool("quiet") {
			fmt.Printf("Overall Progress: %.2f%%\n", renderProgressResponse.OverallProgress*100)
		}

		if renderProgressResponse.Done {
			if !c.Bool("quiet") {
				fmt.Println("Rendering completed successfully.")
			}
			break
		}

		time.Sleep(5 * time.Second)
	}

	fmt.Println("https://" + renderResponse.BucketName + ".s3." + renderInputRequest.Region + ".amazonaws.com/renders/" + renderResponse.RenderId + "/out.mp4")

	return nil
}
