package templates

const resetTmpl = `
<div>
	{{# Sandbox }}<p style="font-size:14px; color:#000000; margin:0 0 12px 0; font-weight: 600;">**NOTE: #sandboxLife #dontFreakOutThisIsATest **</p>{{/ Sandbox }}
	<p style="font-size:14px; color:#000000; margin:0 0 12px 0;">We have received a request to reset your password.</p>
	<p style="font-size:14px; color:#000000; margin:0;">Please click the link below to reset your password, the link will be valid for 24 hours.<br><a href="http://www.swayops.com/loginscreen">http://www.swayops.com/loginscreen</a></p>
	<p style="font-size:14px; color:#000000; margin:0;">If this request was not intended, please disregard this message. </a></p>
	<p style="font-size:14px; color:#000000; margin:0;">Kind regards,</p>
	<p style="font-size:14px; color:#000000; margin:0;">~ Sway Ops Engineering Team</p>
</div>
`

var ResetPassword = MustacheMust(resetTmpl)
