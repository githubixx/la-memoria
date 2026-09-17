// Command bookmarker-hash generates a Bookmarker-compatible Argon2id password verifier.
package main

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/githubixx/la-memoria/bookmarker/usecase"
	"golang.org/x/term"
)

func main() {
	password, err := readPassword()
	if err != nil {
		fmt.Fprintf(os.Stderr, "read password: %v\n", err)
		os.Exit(1)
	}
	verifier, err := usecase.GeneratePasswordHash(password)
	if err != nil {
		fmt.Fprintf(os.Stderr, "generate password verifier: %v\n", err)
		os.Exit(1)
	}
	fmt.Fprintln(os.Stdout, verifier)
}

func readPassword() (string, error) {
	if term.IsTerminal(int(os.Stdin.Fd())) {
		fmt.Fprint(os.Stderr, "Administrator password: ")
		password, err := term.ReadPassword(int(os.Stdin.Fd()))
		fmt.Fprintln(os.Stderr)
		if err != nil {
			return "", err
		}
		return string(password), nil
	}

	reader := bufio.NewReader(os.Stdin)
	password, err := reader.ReadString('\n')
	if err != nil {
		return "", err
	}
	return strings.TrimSuffix(strings.TrimSuffix(password, "\n"), "\r"), nil
}
