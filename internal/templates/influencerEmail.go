package templates

const infEmailTmpl = `
<div>
	<p style="font-size:14px; color:#000000; margin:0 0 12px 0;">
		Hey {{Name}},
	</p>
	<p style="font-size:14px; color:#000000; margin:0 0 12px 0;">
		Hope you're doing well. I wanted to reach out as we are excited to announce there are new Sways that requested your participation:
	</p>
	<p style="font-size:14px; color:#000000; margin:0 0 12px 0;">
		<table border="0" cellpadding="20" cellspacing="0" width="600" style="font-size:14px;">
		{{#deal}}
		<tr>
			<th align="left"></th>
			<th align="left">Company:</th>
			<th align="left">Campaign name:</th>
		</tr>
	    <tr>
	    	<td align="left" valign="middle"><img src="https://dash.swayops.com{{CampaignImage}}" height="50"></td>
	    	<td align="left" valign="middle">{{Company}}</td>
	    	<td align="left" valign="middle">{{CampaignName}}</td>
	    </tr>
	    {{/deal}}
		</table>
	</p>

	<p style="font-size:14px; color:#000000; margin:0 0 12px 0;">
		In order to access these you simply need to go into our influencer app at <a href="https://inf.swayops.com/login">https://inf.swayops.com/login</a> and hit the "Accept Endorsement" button for the above deal in your feed.<br/> Feel free to call or email me with any questions.
	</p>
	<p style="font-size:14px; color:#000000; margin:0 0 12px 0;">
		All the best,<br/>
		~ Karlie M<br/>
	</p>
	<p style="font-size:14px; color:#000000; margin:0 0 12px 0;">
		<img src="http://swayops.com/swayEmailLogo.png" alt="" height="40" />
		<br/>
		Karlie@SwayOps.com | Office: 650-667-7929 | Address: 4461 Crossvine Dr, Prosper TX, 75078
	</p>

</div>
`

const infCmpEmail = `
<div>
	<p style="font-size:14px; color:#000000; margin:0 0 12px 0;">
		Hey {{Name}},
	</p>
	<p style="font-size:14px; color:#000000; margin:0 0 12px 0;">
		Hope you're doing well. Just wanted to reach out as we are excited to announce there is a new Sway that requested your participation and I wanted to get your interest level:
	</p>
	<p style="font-size:14px; color:#000000; margin:0 0 12px 0;">
		<table border="0" cellpadding="20" cellspacing="0" width="600" style="font-size:14px;">
		{{#deal}}
		<tr>
			<th align="left"></th>
			<th align="left">Company:</th>
			<th align="left">Campaign name:</th>
		</tr>
	    <tr>
	    	<td align="left" valign="middle"><img src="https://dash.swayops.com{{CampaignImage}}" height="50"></td>
	    	<td align="left" valign="middle">{{Company}}</td>
	    	<td align="left" valign="middle">{{CampaignName}}</td>
	    </tr>
	    {{/deal}}
		</table>
	</p>
	<p style="font-size:14px; color:#000000; margin:0 0 12px 0;">
		In order to access this deal you simply need to go into our influencer app at <a href="https://inf.swayops.com/login">https://inf.swayops.com/login</a> and hit the "Accept Endorsement" button for the above deal in your feed.<br/> Feel free to call or email me with any questions.
	</p>
	<p style="font-size:14px; color:#000000; margin:0 0 12px 0;">
		All the best,<br/>
		~ Karlie M<br/>
	</p>
	<p style="font-size:14px; color:#000000; margin:0 0 12px 0;">
		<img src="http://swayops.com/swayEmailLogo.png" alt="" height="40" />
		<br/>
		Karlie@SwayOps.com | Office: 650-667-7929 | Address: 4461 Crossvine Dr, Prosper TX, 75078
	</p>
</div>
`

const headsUpEmail = `
<div>
	<p style="font-size:14px; color:#000000; margin:0 0 12px 0;">
		Hey {{Name}},
	</p>
	<p style="font-size:14px; color:#000000; margin:0 0 12px 0;">
		Just wanted to reach out and let you know that you only have 7 days left to complete the deal for {{Company}}. After the 7 days, we will unfortunately be forced to retract the deal from you!
	</p>
	<p style="font-size:14px; color:#000000; margin:0 0 12px 0;">
		If you would like to access the deal requirements, please visit <a href="https://inf.swayops.com/login">https://inf.swayops.com/login</a> <br/> Feel free to call or email me with any questions.
	</p>
	<p style="font-size:14px; color:#000000; margin:0 0 12px 0;">
		All the best,<br/>
		~ Karlie M<br/>
	</p>
	<p style="font-size:14px; color:#000000; margin:0 0 12px 0;">
		<img src="http://swayops.com/swayEmailLogo.png" alt="" height="40" />
		<br/>
		Karlie@SwayOps.com | Office: 650-667-7929 | Address: 4461 Crossvine Dr, Prosper TX, 75078
	</p>
</div>
`

const timeOutEmail = `
<div>
	<p style="font-size:14px; color:#000000; margin:0 0 12px 0;">
		Hey {{Name}},
	</p>
	<p style="font-size:14px; color:#000000; margin:0 0 12px 0;">
		We regret to inform you that we have been forced to retract the deal for {{Company}} from you. Due to strict campaign requirements, there was a limit on the number of days we allow a deal to be left incomplete.
	</p>
	<p style="font-size:14px; color:#000000; margin:0 0 12px 0;">
		If you would like to access more deals, please visit <a href="https://inf.swayops.com/login">https://inf.swayops.com/login</a> .
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

const checkTmpl = `
<div>
	<p style="font-size:14px; color:#000000; margin:0 0 12px 0;">
		Congratulations {{Name}}!
	</p>
	<p style="font-size:14px; color:#000000; margin:0 0 12px 0;">
		We have just sent out your check for ${{Payout}}! It will take approximately {{Delivery}} business days to arrive.
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

const completionTmpl = `
<div>
	<p style="font-size:14px; color:#000000; margin:0 0 12px 0;">
		Congratulations {{Name}}!
	</p>
	<p style="font-size:14px; color:#000000; margin:0 0 12px 0;">
		Your deal for {{Company}} has just been approved! Keep an eye on your Sway Stats on the <a href="https://inf.swayops.com/login">Influencer Dashboard</a> as you receive earnings based on engagements your post receives.
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

const campaignStatusEmail = `
<div>
	<p style="font-size:14px; color:#000000; margin:0 0 12px 0;">
		Hey {{Name}},
	</p>
	<p style="font-size:14px; color:#000000; margin:0 0 12px 0;">
		We regret to inform you that the deal you had accepted for {{Company}} is no longer available.
	</p>
	<p style="font-size:14px; color:#000000; margin:0 0 12px 0;">
		If you would like to access more deals, please visit <a href="https://inf.swayops.com/login">https://inf.swayops.com/login</a> .
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

const dealRejectionEmail = `
<div>
	<p style="font-size:14px; color:#000000; margin:0 0 12px 0;">
		Hi {{Name}},
	</p>
	<p style="font-size:14px; color:#000000; margin:0 0 12px 0;">
		Your most recent deal post ( {{url}} ) is missing a required item. Unfortunately our engine can't pickup your completed deal because of this. Please double check that you included the required <b>{{reason}}</b> in your post and the system will automatically authorize your post.	</p>
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

const privateEmail = `
<div>
	<p style="font-size:14px; color:#000000; margin:0 0 12px 0;">
		Hi {{Name}},
	</p>
	<p style="font-size:14px; color:#000000; margin:0 0 12px 0;">
		Our engine detected that one or more of your social media profiles is now private. Until your profile is made public, we cannot track your stats or detect when you complete deals. Let us know if you have any questions. Happy to help 	<p style="font-size:14px; color:#000000; margin:0 0 12px 0;">
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
	InfluencerEmail        = MustacheMust(infEmailTmpl)
	InfluencerCmpEmail     = MustacheMust(infCmpEmail)
	InfluencerHeadsUpEmail = MustacheMust(headsUpEmail)
	InfluencerTimeoutEmail = MustacheMust(timeOutEmail)
	CheckEmail             = MustacheMust(checkTmpl)
	DealCompletionEmail    = MustacheMust(completionTmpl)
	CampaignStatusEmail    = MustacheMust(campaignStatusEmail)
	DealRejectionEmail     = MustacheMust(dealRejectionEmail)
	PrivateEmail           = MustacheMust(privateEmail)
)