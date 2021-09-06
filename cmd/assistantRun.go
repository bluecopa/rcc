package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/robocorp/rcc/cloud"
	"github.com/robocorp/rcc/common"
	"github.com/robocorp/rcc/operations"
	"github.com/robocorp/rcc/pathlib"
	"github.com/robocorp/rcc/pretty"
	"github.com/robocorp/rcc/robot"
	"github.com/robocorp/rcc/xviper"

	"github.com/spf13/cobra"
)

var assistantRunCmd = &cobra.Command{
	Use:     "run",
	Aliases: []string{"r"},
	Short:   "Robot Assistant run",
	Long:    "Robot Assistant run.",
	Run: func(cmd *cobra.Command, args []string) {
		var status, reason string
		status, reason = "ERROR", "UNKNOWN"
		elapser := common.Stopwatch("Robot Assistant startup lasted")
		if common.DebugFlag {
			defer common.Stopwatch("Robot Assistant run lasted").Report()
		}
		defer xviper.RunMinutes().Done()
		account := operations.AccountByName(AccountName())
		if account == nil {
			pretty.Exit(1, "Could not find account by name: %q", AccountName())
		}
		client, err := cloud.NewClient(account.Endpoint)
		if err != nil {
			pretty.Exit(2, "Could not create client for endpoint: %v, reason: %v", account.Endpoint, err)
		}
		reason = "START_FAILURE"
		cloud.BackgroundMetric(common.ControllerIdentity(), "rcc.assistant.run.start", elapser.Elapsed().String())
		defer func() {
			cloud.BackgroundMetric(common.ControllerIdentity(), "rcc.assistant.run.stop", reason)
		}()
		common.Timeline("start assistant run cloud call started")
		assistant, err := operations.StartAssistantRun(client, account, workspaceId, assistantId, useEcc)
		common.Timeline("start assistant run cloud call completed")
		if err != nil {
			pretty.Exit(3, "Could not run assistant, reason: %v", err)
		}
		cancel := make(chan bool)
		go operations.BackgroundAssistantHeartbeat(cancel, client, account, workspaceId, assistantId, assistant.RunId)
		if assistant != nil && len(assistant.RunId) > 0 {
			defer func() {
				close(cancel)
				common.Debug("Signaling cloud with status %v with reason %v.", status, reason)
				err := operations.StopAssistantRun(client, account, workspaceId, assistantId, assistant.RunId, status, reason)
				common.Error("Stop assistant", err)
			}()
		}
		common.Debug("Robot Assistant run-id is %v.", assistant.RunId)
		common.Debug("With task '%v' from zip %v.", assistant.TaskName, assistant.Zipfile)
		sentinelTime := time.Now()
		workarea := filepath.Join(os.TempDir(), fmt.Sprintf("workarea%x", common.When))
		defer os.RemoveAll(workarea)
		common.Debug("Using temporary workarea: %v", workarea)
		reason = "UNZIP_FAILURE"
		err = operations.Unzip(workarea, assistant.Zipfile, false, true)
		if err != nil {
			pretty.Exit(4, "Error: %v", err)
		}
		reason = "SETUP_FAILURE"
		targetRobot := robot.DetectConfigurationName(workarea)
		simple, config, todo, label := operations.LoadTaskWithEnvironment(targetRobot, assistant.TaskName, forceFlag)
		artifactDir := config.ArtifactDirectory()
		if len(copyDirectory) > 0 && len(artifactDir) > 0 {
			err := os.MkdirAll(copyDirectory, 0o755)
			if err == nil {
				defer pathlib.Walk(artifactDir, pathlib.IgnoreOlder(sentinelTime).Ignore, TargetDir(copyDirectory).OverwriteBack)
			}
		}
		if common.DebugFlag {
			elapser.Report()
		}

		defer func() {
			cloud.BackgroundMetric(common.ControllerIdentity(), "rcc.assistant.run.timeline.uploaded", elapser.Elapsed().String())
		}()
		defer func() {
			common.Timeline("publish artifacts")
			publisher := operations.ArtifactPublisher{
				Client:          client,
				ArtifactPostURL: assistant.ArtifactURL,
				ErrorCount:      0,
			}
			common.Log("Pushing artifacts to Cloud.")
			pathlib.Walk(artifactDir, pathlib.IgnoreDirectories, publisher.Publish)
			if publisher.ErrorCount > 0 {
				reason = "UPLOAD_FAILURE"
				pretty.Exit(5, "Error: Some of uploads failed.")
			}
		}()

		cloud.BackgroundMetric(common.ControllerIdentity(), "rcc.assistant.run.timeline.setup", elapser.Elapsed().String())
		defer func() {
			cloud.BackgroundMetric(common.ControllerIdentity(), "rcc.assistant.run.timeline.executed", elapser.Elapsed().String())
		}()
		reason = "ROBOT_FAILURE"
		operations.SelectExecutionModel(captureRunFlags(true), simple, todo.Commandline(), config, todo, label, false, assistant.Environment)
		pretty.Ok()
		status, reason = "OK", "PASS"
	},
}

var (
	useEcc bool
)

func init() {
	assistantCmd.AddCommand(assistantRunCmd)
	assistantRunCmd.Flags().StringVarP(&workspaceId, "workspace", "w", "", "Workspace id to get assistant information.")
	assistantRunCmd.MarkFlagRequired("workspace")
	assistantRunCmd.Flags().StringVarP(&assistantId, "assistant", "a", "", "Assistant id to execute.")
	assistantRunCmd.MarkFlagRequired("assistant")
	assistantRunCmd.Flags().StringVarP(&copyDirectory, "copy", "c", "", "Location to copy changed artifacts from run (optional).")
	assistantRunCmd.Flags().BoolVarP(&useEcc, "ecc", "", false, "DO NOT USE! INTERNAL EXPERIMENT!")
	assistantRunCmd.Flags().StringVarP(&common.HolotreeSpace, "space", "s", "user", "Client specific name to identify this environment.")
}
