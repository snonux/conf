[Unit]
Description=Hourly unattended-upgrade check (updates once per day)

[Timer]
OnBootSec=10min
OnCalendar=*-*-* *:45:00
Persistent=true

[Install]
WantedBy=timers.target
