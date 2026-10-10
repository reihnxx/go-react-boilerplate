package main

import (
	"context"
	"crypto/rand"
	"errors"
	"flag"
	"fmt"
	"math/big"
	"os"
	"strings"

	"golang.org/x/term"

	"github.com/reihnxx/go-react-boilerplate/backend/internal/app"
	"github.com/reihnxx/go-react-boilerplate/backend/internal/platform/httpx"
	"github.com/reihnxx/go-react-boilerplate/backend/internal/platform/password"
)

func createAdmin(ctx context.Context, a *app.App, args []string) error {
	fs := flag.NewFlagSet("create-admin", flag.ContinueOnError)
	email := fs.String("email", "", "admin email (required)")
	username := fs.String("username", "admin", "admin username")
	name := fs.String("name", "Super Admin", "display name")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *email == "" {
		return errors.New("-email is required")
	}

	pw := os.Getenv("ADMIN_PASSWORD")
	if pw == "" {
		var err error
		if pw, err = promptPassword(); err != nil {
			return err
		}
	}

	id, err := a.Users.CreateSuperAdmin(ctx, *name, *username, *email, pw)
	if err != nil {
		return describe(err)
	}
	fmt.Printf("super-admin created: %s (%s)\n", *username, id)
	return nil
}

func promptPassword() (string, error) {
	fd := int(os.Stdin.Fd())
	if !term.IsTerminal(fd) {
		return "", errors.New("set ADMIN_PASSWORD or run in an interactive terminal")
	}
	fmt.Print("Password: ")
	p1, err := term.ReadPassword(fd)
	fmt.Println()
	if err != nil {
		return "", err
	}
	fmt.Print("Confirm password: ")
	p2, err := term.ReadPassword(fd)
	fmt.Println()
	if err != nil {
		return "", err
	}
	if string(p1) != string(p2) {
		return "", errors.New("passwords do not match")
	}
	return string(p1), nil
}

// seed creates development users. Their password comes from SEED_PASSWORD or
// is generated and printed once.
func seed(ctx context.Context, a *app.App) error {
	pw := os.Getenv("SEED_PASSWORD")
	generated := pw == ""
	if generated {
		pw = randomPassword()
	}
	if msg := password.Validate(pw); msg != "" {
		return errors.New("SEED_PASSWORD: " + msg)
	}

	created := 0
	try := func(name, username, email string, active bool, roles ...string) error {
		_, err := a.Users.CreateWithRoleSlugs(ctx, name, username, email, pw, active, roles...)
		if isDuplicate(err) {
			return nil // already seeded
		}
		if err == nil {
			created++
		}
		return describe(err)
	}

	if err := try("User Demo", "user", "user@example.com", true, "user"); err != nil {
		return err
	}
	for i := 1; i <= 20; i++ {
		if err := try(fmt.Sprintf("Pengguna Demo %02d", i), fmt.Sprintf("demo%02d", i),
			fmt.Sprintf("demo%02d@example.com", i), i%4 != 0, "user"); err != nil {
			return err
		}
	}

	fmt.Printf("seed: %d user(s) created\n", created)
	if created > 0 && generated {
		fmt.Printf("seed: password for seeded users: %s\n", pw)
	}
	return nil
}

func randomPassword() string {
	const letters = "abcdefghijkmnpqrstuvwxyzABCDEFGHJKLMNPQRSTUVWXYZ"
	const digits = "23456789"
	var b strings.Builder
	pick := func(set string) {
		n, _ := rand.Int(rand.Reader, big.NewInt(int64(len(set))))
		b.WriteByte(set[n.Int64()])
	}
	for range 10 {
		pick(letters)
	}
	for range 4 {
		pick(digits)
	}
	return b.String()
}

// isDuplicate reports a unique-username/email violation.
func isDuplicate(err error) bool {
	var apiErr *httpx.Error
	if !errors.As(err, &apiErr) {
		return false
	}
	for _, msg := range apiErr.Fields {
		if strings.Contains(msg, "sudah digunakan") {
			return true
		}
	}
	return false
}

// describe turns validation errors into readable CLI messages.
func describe(err error) error {
	var apiErr *httpx.Error
	if !errors.As(err, &apiErr) || len(apiErr.Fields) == 0 {
		return err
	}
	parts := make([]string, 0, len(apiErr.Fields))
	for f, m := range apiErr.Fields {
		parts = append(parts, f+": "+m)
	}
	return errors.New(strings.Join(parts, "; "))
}
