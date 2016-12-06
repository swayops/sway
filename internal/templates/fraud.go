package templates

const fraudTmpl = `
<div>
	<p style="font-size:14px; color:#000000; margin:0 0 12px 0;">Fraud detected!</p>
	<p style="font-size:14px; color:#000000; margin:0;"><b>Message:</b> {{msg}} </a></p><br>
	<p style="font-size:14px; color:#000000; margin:0;">Kind regards,</p>
	<p style="font-size:14px; color:#000000; margin:0;">The SwayOps Server.</p>
</div>
`

var FraudEmail = MustacheMust(fraudTmpl)
