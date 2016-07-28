package templates

const resetTmpl = `
<div>
	{{# Sandbox }}<p style="font-size:14px; color:#000000; margin:0 0 12px 0; font-weight: 600;">**NOTE: #sandboxLife #dontFreakOutThisIsATest **</p>{{/ Sandbox }}
	<p style="font-size:14px; color:#000000; margin:0 0 12px 0;">the second to scraped accounts after 24 hours or whatever
</p>
	<p style="font-size:14px; color:#000000; margin:0;">Test</p>

</div>
`

var ResetPassword = MustacheMust(resetTmpl)
