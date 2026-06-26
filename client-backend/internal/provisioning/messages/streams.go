package messages

const DefaultCommandStreamName = "RACK_COMMANDS"

func CommandStreamSubjects() []string {
	return []string{
		"dc.*.provision.request",
		"dc.*.server.power",
		"dc.*.hardware.sync",
		"dc.*.workflow.cancel",
	}
}
