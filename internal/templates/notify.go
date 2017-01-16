package templates

const notifyTmpl = `
<div>
	<p style="font-size:14px; color:#000000; margin:0 0 12px 0;">Friendly notification!</p>
	<p style="font-size:14px; color:#000000; margin:0;"><b>Message:</b> {{msg}} </a></p><br>
	<p style="font-size:14px; color:#000000; margin:0;">Kind regards,</p>
	<p style="font-size:14px; color:#000000; margin:0;">The SwayOps Server.</p>
</div>
`

const notifyPerk = `
<div>
	<p style="font-size:14px; color:#000000; margin:0 0 12px 0;">
		Hi {{Name}},
	</p>
	<p style="font-size:14px; color:#000000; margin:0 0 12px 0;">
		We are emailing to inform you that we have just received your shipment of {{Perk}}. As a result, all campaigns associated with the shipment are now live.
	</p>
	<p style="font-size:14px; color:#000000; margin:0 0 12px 0;">
		Feel free to call or email me with any questions.
	</p>
	<p style="font-size:14px; color:#000000; margin:0 0 12px 0;">
		Regards,<br/>
		~ Karlie M<br/>
	</p>
	<p style="font-size:14px; color:#000000; margin:0 0 12px 0;">
		<img src="http://swayops.com/swayEmailLogo.png" alt="" height="40" />
		<br/>
		Karlie@SwayOps.com | Office: 650-667-7929 | Address: 4461 Crossvine Dr, Prosper TX, 75078
	</p>
</div>
`

var (
	NotifyEmail     = MustacheMust(notifyTmpl)
	NotifyPerkEmail = MustacheMust(notifyPerk)
)
