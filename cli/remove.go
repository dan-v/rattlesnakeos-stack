package cli

import (
	"fmt"

	"github.com/dan-v/rattlesnakeos-stack/stack"
	"github.com/fatih/color"
	"github.com/manifoldco/promptui"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

func init() {
	rootCmd.AddCommand(removeCmd)

	removeCmd.Flags().StringVarP(&name, "name", "n", "",
		"name of stack you'd like to remove.")

	removeCmd.Flags().StringVarP(&region, "region", "r", "",
		"region where stack was deployed to (e.g. us-west-2)")
}

var removeCmd = &cobra.Command{
	Use:   "remove",
	Short: "Remove all AWS infrastructure used for building RattlesnakeOS",
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

		log.Println("Details of stack to be deleted:")
		fmt.Println("Stack name:", name)
		fmt.Println("Stack region:", region)
		fmt.Println("")

		color.Red("This is a destructive action! All S3 buckets will be removed and all data will be destroyed. " +
			"Make sure to backup anything you might want to keep!")
		prompt := promptui.Prompt{
			Label:     fmt.Sprintf("This will remove all AWS infrastructure for stack %v. Do you want to continue ", viper.GetString("name")),
			IsConfirm: true,
		}
		_, err := prompt.Run()
		if err != nil {
			log.Fatalf("Exiting %v", err)
		}

		s, err := stack.NewAWSStack(&stack.AWSStackConfig{
			Name:   name,
			Region: region,
		})
		if err != nil {
			log.Fatal(err)
		}
		if err := s.Destroy(); err != nil {
			log.Fatal(err)
		}
	},
}
