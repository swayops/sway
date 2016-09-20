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
	    	<td align="left" valign="middle"><img src="{{CampaignImage}}" height="50"></td>
	    	<td align="left" valign="middle">{{Company}}</td>
	    	<td align="left" valign="middle">{{CampaignName}}</td>
	    </tr>
	    {{/deal}}
		</table>
	</p>

	<p style="font-size:14px; color:#000000; margin:0 0 12px 0;">
		In order to access these you simply need to go into our influencer app at "INSERT SIGN IN URL WHEN ITS TIME" and hit the "Accept Endorsement" button.<br/> Feel free to call or email me with any questions.
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
	    	<td align="left" valign="middle"><img src="{{CampaignImage}}" height="50"></td>
	    	<td align="left" valign="middle">{{Company}}</td>
	    	<td align="left" valign="middle">{{CampaignName}}</td>
	    </tr>
	    {{/deal}}
		</table>
	</p>
	<p style="font-size:14px; color:#000000; margin:0 0 12px 0;">
		In order to access this deal you simply need to go into our influencer app at "INSERT SIGN UP URL HEREE when we get it" and hit the "Accept Endorsement" button.<br/> Feel free to call or email me with any questions.
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

var (
	InfluencerEmail    = MustacheMust(infEmailTmpl)
	InfluencerCmpEmail = MustacheMust(infCmpEmail)
)
