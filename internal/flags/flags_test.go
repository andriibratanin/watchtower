package flags

import (
	"io/ioutil"
	"os"
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEnvConfig_Defaults(t *testing.T) {
	// Unset testing environments own variables, since those are not what is under test
	os.Unsetenv("DOCKER_TLS_VERIFY")
	os.Unsetenv("DOCKER_HOST")

	cmd := new(cobra.Command)
	SetDefaults()
	RegisterDockerFlags(cmd)

	err := EnvConfig(cmd)
	require.NoError(t, err)

	assert.Equal(t, "unix:///var/run/docker.sock", os.Getenv("DOCKER_HOST"))
	assert.Equal(t, "", os.Getenv("DOCKER_TLS_VERIFY"))
	// Re-enable this test when we've moved to github actions.
	// assert.Equal(t, DockerAPIMinVersion, os.Getenv("DOCKER_API_VERSION"))
}

func TestEnvConfig_Custom(t *testing.T) {
	cmd := new(cobra.Command)
	SetDefaults()
	RegisterDockerFlags(cmd)

	err := cmd.ParseFlags([]string{"--host", "some-custom-docker-host", "--tlsverify", "--api-version", "1.99"})
	require.NoError(t, err)

	err = EnvConfig(cmd)
	require.NoError(t, err)

	assert.Equal(t, "some-custom-docker-host", os.Getenv("DOCKER_HOST"))
	assert.Equal(t, "1", os.Getenv("DOCKER_TLS_VERIFY"))
	// Re-enable this test when we've moved to github actions.
	// assert.Equal(t, "1.99", os.Getenv("DOCKER_API_VERSION"))
}

func TestGetSecretsFromFilesWithString(t *testing.T) {
	value := "supersecretstring"

	err := os.Setenv("WATCHTOWER_NOTIFICATION_EMAIL_SERVER_PASSWORD", value)
	require.NoError(t, err)
	defer os.Unsetenv("WATCHTOWER_NOTIFICATION_EMAIL_SERVER_PASSWORD")

	testGetSecretsFromFiles(t, "notification-email-server-password", value)
}

func TestGetSecretsFromFilesWithFile(t *testing.T) {
	value := "megasecretstring"

	// Create the temporary file which will contain a secret.
	file, err := ioutil.TempFile(os.TempDir(), "watchtower-")
	require.NoError(t, err)
	defer os.Remove(file.Name()) // Make sure to remove the temporary file later.

	// Write the secret to the temporary file.
	secret := []byte(value)
	_, err = file.Write(secret)
	require.NoError(t, err)

	err = os.Setenv("WATCHTOWER_NOTIFICATION_EMAIL_SERVER_PASSWORD", file.Name())
	require.NoError(t, err)
	defer os.Unsetenv("WATCHTOWER_NOTIFICATION_EMAIL_SERVER_PASSWORD")

	testGetSecretsFromFiles(t, "notification-email-server-password", value)
}

func TestGetSliceSecretsFromFiles(t *testing.T) {
	values := []string{"entry2", "", "entry3"}

	// Create the temporary file which will contain a secret.
	file, err := ioutil.TempFile(os.TempDir(), "watchtower-")
	require.NoError(t, err)
	defer os.Remove(file.Name()) // Make sure to remove the temporary file later.

	// Write the secret to the temporary file.
	for _, value := range values {
		_, err = file.WriteString("\n" + value)
		require.NoError(t, err)
	}
	file.Close()

	testGetSecretsFromFiles(t, "notification-url", `[entry1,entry2,entry3]`,
		`--notification-url`, "entry1",
		`--notification-url`, file.Name())
}

func testGetSecretsFromFiles(t *testing.T, flagName string, expected string, args ...string) {
	cmd := new(cobra.Command)
	SetDefaults()
	RegisterNotificationFlags(cmd)
	require.NoError(t, cmd.ParseFlags(args))
	GetSecretsFromFiles(cmd)
	flag := cmd.PersistentFlags().Lookup(flagName)
	require.NotNil(t, flag)
	value := flag.Value.String()

	assert.Equal(t, expected, value)
}

func TestHTTPAPIPeriodicPollsFlag(t *testing.T) {
	cmd := new(cobra.Command)
	SetDefaults()
	RegisterDockerFlags(cmd)
	RegisterSystemFlags(cmd)

	err := cmd.ParseFlags([]string{"--http-api-periodic-polls"})
	require.NoError(t, err)

	periodicPolls, err := cmd.PersistentFlags().GetBool("http-api-periodic-polls")
	require.NoError(t, err)

	assert.Equal(t, true, periodicPolls)
}

func TestIsFile(t *testing.T) {
	assert.False(t, isFile("https://google.com"), "an URL should never be considered a file")
	assert.True(t, isFile(os.Args[0]), "the currently running binary path should always be considered a file")
}

func TestReadFlags(t *testing.T) {
	logrus.StandardLogger().ExitFunc = func(_ int) { t.FailNow() }

}

func TestProcessFlagAliases(t *testing.T) {
	logrus.StandardLogger().ExitFunc = func(_ int) { t.FailNow() }
	cmd := new(cobra.Command)
	SetDefaults()
	RegisterDockerFlags(cmd)
	RegisterSystemFlags(cmd)
	RegisterNotificationFlags(cmd)

	require.NoError(t, cmd.ParseFlags([]string{
		`--porcelain`, `v1`,
		`--interval`, `10`,
	}))
	flags := cmd.Flags()
	ProcessFlagAliases(flags)

	urls, _ := flags.GetStringArray(`notification-url`)
	assert.Contains(t, urls, `logger://`)

	logStdout, _ := flags.GetBool(`notification-log-stdout`)
	assert.True(t, logStdout)

	report, _ := flags.GetBool(`notification-report`)
	assert.True(t, report)

	template, _ := flags.GetString(`notification-template`)
	assert.Equal(t, `porcelain.v1.summary-no-log`, template)

	sched, _ := flags.GetString(`schedule`)
	assert.Equal(t, `@every 10s`, sched)
}

func TestProcessFlagAliasesSchedAndInterval(t *testing.T) {
	logrus.StandardLogger().ExitFunc = func(_ int) { panic(`FATAL`) }
	cmd := new(cobra.Command)
	SetDefaults()
	RegisterDockerFlags(cmd)
	RegisterSystemFlags(cmd)
	RegisterNotificationFlags(cmd)

	require.NoError(t, cmd.ParseFlags([]string{`--schedule`, `@hourly`, `--interval`, `10`}))
	flags := cmd.Flags()

	assert.PanicsWithValue(t, `FATAL`, func() {
		ProcessFlagAliases(flags)
	})
}

func TestProcessFlagAliasesScheduleFromEnvironment(t *testing.T) {
	cmd := new(cobra.Command)

	err := os.Setenv("WATCHTOWER_SCHEDULE", `@hourly`)
	require.NoError(t, err)
	defer os.Unsetenv("WATCHTOWER_SCHEDULE")

	SetDefaults()
	RegisterDockerFlags(cmd)
	RegisterSystemFlags(cmd)
	RegisterNotificationFlags(cmd)

	require.NoError(t, cmd.ParseFlags([]string{}))
	flags := cmd.Flags()
	ProcessFlagAliases(flags)

	sched, _ := flags.GetString(`schedule`)
	assert.Equal(t, `@hourly`, sched)
}

func TestProcessFlagAliasesInvalidPorcelaineVersion(t *testing.T) {
	logrus.StandardLogger().ExitFunc = func(_ int) { panic(`FATAL`) }
	cmd := new(cobra.Command)
	SetDefaults()
	RegisterDockerFlags(cmd)
	RegisterSystemFlags(cmd)
	RegisterNotificationFlags(cmd)

	require.NoError(t, cmd.ParseFlags([]string{`--porcelain`, `cowboy`}))
	flags := cmd.Flags()

	assert.PanicsWithValue(t, `FATAL`, func() {
		ProcessFlagAliases(flags)
	})
}
