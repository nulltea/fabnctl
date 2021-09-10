package term

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/manifoldco/promptui"
	"github.com/spf13/viper"
)

var (
	ErrRemoteCmdFailed = fmt.Errorf("remote command failed")
)

// PromptStderrView asks whether the full log output from stderr should be viewed.
func PromptStderrView(stderr io.Reader) bool {
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
			io.Copy(os.Stderr, stderr)
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
func WrapWithStderrViewPrompt(err error, stderr io.Reader, printErrPriorPrompt bool) error {
	var buffer bytes.Buffer

	if err == nil || stderr == nil {
		return err
	}

	if size, err := io.Copy(&buffer, stderr); size == 0 || err != nil {
		return err
	}

	if printErrPriorPrompt {
		fmt.Println(viper.GetString("cli.error_emoji"), "Error:", err)
	}

	if PromptStderrView(&buffer) {
		return err
	}

	return nil
}

// ErrFromStderr parses stderr to find last error message, which would be returned as error or <nil> otherwise.
func ErrFromStderr(stderr bytes.Buffer) error {
	if errMsg := strings.Replace(GetLastLine(&stderr), "Error: ", "", 1); len(errMsg) != 0 {
		return fmt.Errorf("%s: %w", errMsg, ErrRemoteCmdFailed)
	}

	return nil
}

func GetLastLine(reader io.Reader) string {
	var lines []string

	s := bufio.NewScanner(reader)
	for s.Scan() {
		lines = append(lines, s.Text())
	}
	if err := s.Err(); err != nil {
		return ""
	}

	if len(lines) == 0 {
		return ""
	}

	return lines[len(lines) - 1]
}
