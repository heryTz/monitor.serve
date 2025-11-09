package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"sort"
	"time"

	"github.com/goccy/go-yaml"
	"github.com/shirou/gopsutil/process"
)

type Config struct {
	Notifiers struct {
		Mail struct {
			Enabled      bool
			SMTPHost     string `yaml:"smtp_host"`
			SMTPPort     int    `yaml:"smtp_port"`
			SMTPUser     string `yaml:"smtp_user"`
			SMTPPassword string `yaml:"smtp_password"`
			SMTPCrypto   string `yaml:"smtp_crypto"`
			Destinations []struct {
				Name  string
				Email string
			}
		}
	}
}

type ProcInfo struct {
	PID  int32
	Name string
	CPU  float64
	Mem  float32
}

func main() {
	configPath := flag.String("config", "./config.yml", "Path to the config file")

	byte, err := os.ReadFile(*configPath)
	if err != nil {
		panic("config file not found")
	}

	var config Config
	err = yaml.Unmarshal(byte, &config)
	if err != nil {
		panic("failed to parse config file")
	}

	for {
		log.Print("get process infos\n")
		procs, err := process.Processes()
		if err != nil {
			panic("failed to get process")
		}

		for _, p := range procs {
			p.CPUPercent()
		}

		time.Sleep(10 * time.Second)

		var infos []ProcInfo
		notify := false

		for _, p := range procs {
			cpu, _ := p.CPUPercent()
			name, _ := p.Name()
			mem, _ := p.MemoryPercent()
			if cpu > 60 {
				notify = true
				infos = append(infos, ProcInfo{
					PID:  p.Pid,
					Name: name,
					CPU:  cpu,
					Mem:  mem,
				})
			}
		}

		sort.Slice(infos, func(i, j int) bool {
			return infos[i].CPU > infos[j].CPU
		})

		if notify {

			var tableBody string
			end := len(infos)
			if end > 10 {
				end = 10
			}
			var procsToNotify = infos[:end]
			for _, p := range procsToNotify {
				tableBody = fmt.Sprintf("%s<tr><td>%d</td><td>%s</td><td>%.2f%%</td><td>%.2f%%</td></tr>", tableBody, p.PID, p.Name, p.CPU, p.Mem)
			}

			html := fmt.Sprintf(`
<!DOCTYPE html>
<html>
	<head>
		<style>
		table, th, td {
			border: 1px solid black;
			border-collapse: collapse;
		}
		</style>
	</head>
	<body>
		<h1>High CPU Usage Alert</h1>
		<p>Our monitoring system has detected a high CPU usage.</p>
		<table>
			<thead>
				<tr>
					<th>PID</th>
					<th>Name</th>
					<th>CPU</th>
					<th>Mem</th>
				</tr>
			</thead>
			<tbody>
			%s
			</tbody>
		</table>
	</body>
</html>
			`, tableBody)

			if config.Notifiers.Mail.Enabled {
				var recipients []Recipient
				for _, dest := range config.Notifiers.Mail.Destinations {
					recipients = append(recipients, Recipient{Name: dest.Name, Email: dest.Email})
				}
				mailer := NewStdMailer(config.Notifiers.Mail.SMTPHost, config.Notifiers.Mail.SMTPPort, config.Notifiers.Mail.SMTPUser, config.Notifiers.Mail.SMTPPassword, config.Notifiers.Mail.SMTPCrypto)
				err := mailer.Send(MailerInput{
					Subject:    "High CPU Usage Alert",
					Message:    html,
					Recipients: recipients,
				})
				if err != nil {
					log.Printf("failed to send email. %s", err)
				}
			}
		}

	}

}
