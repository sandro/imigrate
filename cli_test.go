package imigrate

import (
	"fmt"
	"os"
	"testing"
)

type commandData struct {
	command string
	steps   int
	version int64
	newName string
}

type TestingMigrator struct {
}

var data commandData

func (o TestingMigrator) Create(name string) {
	data.command = "create"
	data.newName = name
}
func (o TestingMigrator) Up(steps int, version int64) {
	data.command = "up"
	data.steps = steps
	data.version = version
}
func (o TestingMigrator) Down(steps int, version int64) {
	data.command = "down"
	data.steps = steps
	data.version = version
}
func (o TestingMigrator) Redo(steps int, version int64) {
	data.command = "redo"
	data.steps = steps
}
func (o TestingMigrator) Rollback(steps int) {
	data.command = "rollback"
	data.steps = steps
}
func (o TestingMigrator) Status() {
	data.command = "status"
}

func TestCLIArgs(t *testing.T) {
	tests := []struct {
		args     []string
		expected commandData
	}{
		{[]string{"cli", "create", "new_table"}, commandData{"create", 0, 0, "new_table"}},
		{[]string{"cli", "up"}, commandData{"up", -1, 0, ""}},
		{[]string{"cli", "up", "-steps=1"}, commandData{"up", 1, 0, ""}},
		{[]string{"cli", "down"}, commandData{"down", -1, 0, ""}},
		{[]string{"cli", "down", "-steps=2"}, commandData{"down", 2, 0, ""}},
		{[]string{"cli", "redo", "-steps=3"}, commandData{"redo", 3, 0, ""}},
		{[]string{"cli", "rollback", "-steps=4"}, commandData{"rollback", 4, 0, ""}},
		{[]string{"cli", "status", "new_table"}, commandData{"status", 0, 0, ""}},
		{[]string{"cli", "up", "-version=1610069160"}, commandData{"up", -1, 1610069160, ""}},
		{[]string{"cli", "down", "-version=1610069160"}, commandData{"down", -1, 1610069160, ""}},
	}
	mig := TestingMigrator{}
	for _, tt := range tests {
		data = commandData{}
		t.Run(fmt.Sprintf("%s-%d", tt.expected.command, tt.expected.steps), func(t *testing.T) {
			os.Args = tt.args
			CLI(mig)
			if data.command != tt.expected.command || data.steps != tt.expected.steps || data.version != tt.expected.version || data.newName != tt.expected.newName {
				t.Fatalf("expected %#v got %#v\n", tt.expected, data)
			}
		})
	}
}
