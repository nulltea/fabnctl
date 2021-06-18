package util

import (
	"fmt"

	"github.com/manifoldco/promptui"
	"github.com/pkg/errors"
	"github.com/spf13/viper"
)

// PromptStderrView asks whether the full log output from stderr should be viewed.
func PromptStderrView(stderr string) bool {
	prompt := promptui.Prompt{
		Label:    "View full error log?",
		IsConfirm: true,
		Templates: &promptui.PromptTemplates{
			Confirm: "❓ {{ . }} [y/N]: ",
		},
		HideEntered: true,
		Default: "y",
	}

	for {
		answer, err := prompt.Run()

		// Handle os.Interrupt:
		if err != nil && len(answer) == 0 {
			return false
		}

		switch answer {
		case "y", "yes":
			fmt.Println(stderr)
			return true
		case "n", "no":
			return false
		default:
			prompt.Label = "View full error log? Type 'y' (yes) or 'n' (no)"
			prompt.Templates.Confirm = "❓ {{ . }}: "
		}
	}
}

// WrapWithStderrViewPrompt wraps `err` from remote container with `msg`,
// and asks whether the full log output from stderr should be viewed.
func WrapWithStderrViewPrompt(err error, stderr string, msg string) error {
	if err == nil || len(stderr) == 0 {
		return err
	}

	fmt.Println(viper.GetString("cli.error_emoji"), errors.Wrap(err, msg))

	if PromptStderrView(stderr) {
		return err
	}

	return nil
}

// WrapWithStderrViewPromptF wraps `err` from remote container with formatted message`,
// and asks whether the full log output from stderr should be viewed.
func WrapWithStderrViewPromptF(err error, stderr string, format string, v ...interface{}) error {
	return WrapWithStderrViewPrompt(err, stderr, fmt.Sprintf(format, v))
}
