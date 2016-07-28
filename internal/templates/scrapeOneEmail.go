package templates

const resetTmpl = `
<div>
	{{# Sandbox }}<p style="font-size:14px; color:#000000; margin:0 0 12px 0; font-weight: 600;">**NOTE: #sandboxLife #dontFreakOutThisIsATest **</p>{{/ Sandbox }}
	<p style="font-size:14px; color:#000000; margin:0 0 12px 0;">the first one we send to scraped accounts
</p>
	<p style="font-size:14px; color:#000000; margin:0;">Test</p>

</div>
`

var ResetPassword = MustacheMust(resetTmpl)
