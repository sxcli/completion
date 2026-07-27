module sxcli.dev/completion

go 1.26.4

require sxcli.dev/fw v0.3.0

require (
	golang.org/x/sys v0.47.0 // indirect
	sxcli.dev/conf v0.1.1 // indirect
	sxcli.dev/rules v0.0.0 // indirect
)

replace sxcli.dev/fw => ../sxcli-fw

replace sxcli.dev/conf => ../sxcli-conf

replace sxcli.dev/rules => ../sxcli-rules
