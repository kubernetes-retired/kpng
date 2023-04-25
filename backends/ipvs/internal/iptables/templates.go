package iptables

const Template = `
{{- /* NO NEW LINE */ -}}			{{range . }}
{{- /* NO NEW LINE */ -}}					{{template "TableTemplate" .}}
{{/* NEW LINE */ -}}				{{end}}
{{- /* NO NEW LINE */ -}}`

const TableTemplate = `
{{- /* NO NEW LINE */ -}}			*{{js .Table}}
{{/* NEW LINE */ -}}	 			{{range .Chains}}
{{- /* NO NEW LINE */ -}}					{{template "ChainTemplate" .}}
{{- /* NO NEW LINE */ -}} 			{{end}}
{{- /* NO NEW LINE */ -}}	 		{{ range .Rules}}
{{- /* NO NEW LINE */ -}}					{{template "RuleTemplate" .}}
{{- /* NO NEW LINE */ -}}			{{end -}}
{{- /* NO NEW LINE */ -}}			COMMIT
{{- /* NO NEW LINE */ -}}`

const ChainTemplate = `
{{- /* NO NEW LINE */ -}}		 	:{{.}}
{{- /* NO NEW LINE */ -}}				 {{if is_default_chain .}}
{{- /* NO NEW LINE */ -}} 						{{/* WHITE SPACE */}} ACCEPT
{{- /* NO NEW LINE */ -}}				{{else}}
{{- /* NO NEW LINE */ -}}						{{/* WHITE SPACE */}} - 
{{- /* NO NEW LINE */ -}}				{{end}}
{{- /* NO NEW LINE */ -}} 				{{/* WHITE SPACE */}} [0:0]
{{/* NEW LINE */ -}}`

const RuleTemplate = `
{{- /* NO NEW LINE */ -}}				-A {{.From}} 
{{- /* NO NEW LINE */ -}} 				{{template "ProtocolTemplate" .Protocol }}
{{- /* NO NEW LINE */ -}} 				{{range .MatchOptions}}
{{- /* NO NEW LINE */ -}}						{{template "MatchTemplate" .}}
{{- /* NO NEW LINE */ -}}				{{end}}
{{- /* NO NEW LINE */ -}} 				{{/* WHITE SPACE */}} -j
{{- /* NO NEW LINE */ -}}				{{if .To}}
{{- /* NO NEW LINE */ -}}						{{/* WHITE SPACE */}} {{.To}}
{{- /* NO NEW LINE */ -}}				{{else}}
{{- /* NO NEW LINE */ -}}						{{/* WHITE SPACE */}} {{.Target}}
{{- /* NO NEW LINE */ -}}						{{if .TargetOption}}
{{- /* NO NEW LINE */ -}}								{{/* WHITE SPACE */}} {{.TargetOption}}
{{- /* NO NEW LINE */ -}}						{{else}}
{{- /* NO NEW LINE */ -}}						{{end}}
{{- /* NO NEW LINE */ -}}						{{if .TargetOptionValue}}
{{- /* NO NEW LINE */ -}}								{{/* WHITE SPACE */}} {{.TargetOptionValue}}
{{- /* NO NEW LINE */ -}}						{{else}}
{{- /* NO NEW LINE */ -}}						{{end}}
{{- /* NO NEW LINE */ -}}				{{end}}
{{/* NEW LINE */ -}}`

const ProtocolTemplate = `
{{- /* NO NEW LINE */ -}}				{{if .}}
{{- /* NO NEW LINE */ -}}						{{/* WHITE SPACE */}} -p {{js .}}
{{- /* NO NEW LINE */ -}}				{{else}} 
{{- /* NO NEW LINE */ -}}				{{- end -}}
{{- /* NO NEW LINE */ -}}`

const MatchTemplate = `
{{- /* NO NEW LINE */ -}}		 		{{/* WHITE SPACE */}} -m {{js .Module}}
{{- /* NO NEW LINE */ -}}		 		{{if .Inverted}}
{{- /* NO NEW LINE */ -}} 						{{/* WHITE SPACE */}} !
{{- /* NO NEW LINE */ -}}				{{else}}
{{- /* NO NEW LINE */ -}}				{{end}} 
{{- /* NO NEW LINE */ -}}				{{/* WHITE SPACE */}} {{js .ModuleOption}}
{{- /* NO NEW LINE */ -}}				{{if need_quotes .ModuleOption}}
{{- /* NO NEW LINE */ -}} 						{{/* WHITE SPACE */}} "{{.Value}}"
{{- /* NO NEW LINE */ -}} 				{{else}}
{{- /* NO NEW LINE */ -}} 						{{/* WHITE SPACE */}} {{.Value}}
{{- /* NO NEW LINE */ -}} 				{{end}}
{{- /* NO NEW LINE */ -}}`
