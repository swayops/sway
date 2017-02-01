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

const notifyEmptyPerk = `
<div>
	<p style="font-size:14px; color:#000000; margin:0 0 12px 0;">
		Hi {{Name}},
	</p>
	<p style="font-size:14px; color:#000000; margin:0 0 12px 0;">
		We are emailing to inform you that we have run out of perks to send to influencers for your campaign {{Campaign}} (Campaign ID: {{ID}}). As a result, the campaign is no longer sending out new deals to influencers. If you would like to send out more deals, please edit the campaign and increase your perk count, and follow shipping instructions on how to send Sway the product.
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

const notifyBillingEmail = `
<div>
	<p style="font-size:14px; color:#000000; margin:0 0 12px 0;">
		Hi {{Name}},
	</p>
	<p style="font-size:14px; color:#000000; margin:0 0 12px 0;">
		Hope all is well. I am emailing you to remind you that Sway will be re-charging your primary billing method on file for the following campaigns on the first of next month:
	</p>
	<p style="font-size:14px; color:#000000; margin:0 0 12px 0;">
		<table border="0" cellpadding="20" cellspacing="0" width="600" style="font-size:14px;">
		<tr>
			<th align="left">ID:</th>
			<th align="left">Name:</th>
			<th align="left">Amount:</th>
		</tr>
		{{#campaign}}
	    <tr>
	    	<td align="left" valign="middle">{{ID}}</td>
	    	<td align="left" valign="middle">{{Name}}</td>
	    	<td align="left" valign="middle">${{Amount}}</td>
	    </tr>
	    {{/campaign}}
		</table>
	</p>
	<p style="font-size:14px; color:#000000; margin:0 0 12px 0;">
		This ensures that your campaigns will continue to run as normal and influencers can continue to bring constant awareness to your initiatives. Feel free to call or email me with any questions.
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
	NotifyEmail          = MustacheMust(notifyTmpl)
	NotifyPerkEmail      = MustacheMust(notifyPerk)
	NotifyEmptyPerkEmail = MustacheMust(notifyEmptyPerk)
	NotifyBillingEmail   = MustacheMust(notifyBillingEmail)
)
