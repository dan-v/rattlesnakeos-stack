package cmd

import (
	"context"
	"fmt"
	"github.com/dan-v/rattlesnakeos-stack/internal/terraform"
	"github.com/fatih/color"
	"github.com/manifoldco/promptui"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"path/filepath"
)

func removeInit() {
	rootCmd.AddCommand(removeCmd)

	removeCmd.Flags().StringVarP(&name, "name", "n", "",
		"name of stack you'd like to remove.")

	removeCmd.Flags().StringVarP(&region, "region", "r", "",
		"region where stack was deployed to (e.g. us-west-2)")
}

var removeCmd = &cobra.Command{
	Use:   "remove",
	Short: "remove all cloud infrastructure used for OS building",
	Args: func(cmd *cobra.Command, args []string) error {
		if viper.GetString("name") == "" && name == "" {
			return fmt.Errorf("must provide a stack name")
		}
		if viper.GetString("region") == "" && region == "" {
			return fmt.Errorf("must provide a region")
		}
		return nil
	},
	Run: func(cmd *cobra.Command, args []string) {
		if name == "" {
			name = viper.GetString("name")
		}
		if region == "" {
			region = viper.GetString("region")
		}

		log.Println("details of stack to be deleted:")
		fmt.Println("Stack name:", name)
		fmt.Println("Stack region:", region)
		fmt.Println("")

		color.Red("this is a destructive action! all S3 buckets will be removed and all data will be destroyed. " +
			"make sure to backup anything you might want to keep!")
		prompt := promptui.Prompt{
			Label:     fmt.Sprintf("this will remove all AWS infrastructure for stack %v. do you want to continue ", viper.GetString("name")),
			IsConfirm: true,
		}
		_, err := prompt.Run()
		if err != nil {
			log.Fatalf("Exiting %v", err)
		}

		// TODO: this requires directory to already exist
		// TODO: make this configurable and not duplicated
		outputDirFullPath, err := filepath.Abs(fmt.Sprintf("output_%v", viper.GetString("name")))
		if err != nil {
			log.Fatal(err)
		}

		terraformClient, err := terraform.New(outputDirFullPath)
		if err != nil {
			log.Fatalf("failed to create terraform client: %v", err)
		}

		ctx, cancel := context.WithTimeout(context.Background(), terraform.DefaultTerraformDestroyTimeout)
		defer cancel()

		_, err = terraformClient.Destroy(ctx)
		if err != nil {
			log.Fatalf("failed to run terraform destroy: %v", err)
		}
	},
}
