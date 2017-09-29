package templates

const scrapFirstEmail = `
<div>
	<p style="font-size:14px; color:#000000; margin:0 0 12px 0;">
		Hi {{Name}},
	</p>
	<p style="font-size:14px; color:#000000; margin:0 0 12px 0;">
		I have a sponsored post opportunity for you and wanted to reach out. Our company makes software that helps social media influencers get paid for posts sub 15 minutes instead of spending hours negotiating and trading payment/ shipping details back and forth. Here is one of the deals you are currently eligible for:
	</p>
	<p style="font-size:14px; color:#000000; margin:0 0 12px 0;">

		<table border="0" cellpadding="20" cellspacing="0" width="900" style="font-size:14px;">
	    <tr>
	    	<td align="left" valign="middle" style="width: 100px;"><img src="https://dash.swayops.com{{Image}}" height="150"></td>
	    	<td align="left" valign="left">
		    	<b>Brand name:</b> {{Company}} <br/>
		    	<b>Campaign name:</b> {{Campaign}} <br/>
		    	<b>Your likely earnings based on your real followers/avg engagements:</b> ${{Payout}} <br/>
		    	<b>Product perks?:</b> {{Perks}} <br/>
		    	<b>Task description:</b> {{Task}} <br/>
	    	</td>
	    </tr>
		</table>
		
	</p>

	<p style="font-size:14px; color:#000000; margin:0 0 12px 0;">
		In order to access deals you simply need to sign up in our influencer app by <a href="https://inf.swayops.com/signup">Clicking Here</a> and hit the "Accept Endorsement" button inside of the deal you wish to participate in. Feel free to call or email me with any questions, we also have a full wiki on our website as well that explains how fast you get paid, how to calculate your average earnings, etc.
	</p>
	<p style="font-size:14px; color:#000000; margin:0 0 12px 0;">
		If this sounds like something that would interest you please let us know. You can see more info about how our app works at http://SwayOps.com/influencer/ , and if I don't hear from you I will ping you over email when the next brand requests you :) . You can also download our iPhone app from the store named "Sway iOS" to instantly get going.
		Hope to work together soon,<br/>
		~ Karlie M<br/>
	</p>


	<p style="font-size:14px; color:#000000; margin:0 0 12px 0;">
		<img src="http://swayops.com/swayEmailLogo.png" alt="" height="40" />
		<br/>
		Karlie@SwayOps.com | Office: 650-667-7929 | Address: 4461 Crossvine Dr, Prosper TX, 75078
	<br><br>
	Want to be taken off our influencer notification list?: <a href="https://dash.swayops.com/api/v1/optout/{{email}}">Click here</a> 
	</p>
</div>
`

const scrapDealOne = `
<div>
	<p style="font-size:14px; color:#000000; margin:0 0 12px 0;">
		Hey {{Name}},
	</p>
	<p style="font-size:14px; color:#000000; margin:0 0 12px 0;">
		We just wanted to follow up with deals that would be available for you based on my previous email. These are deals you fit based on your fan analytics, follower counts, avg engagement rates, etc. A majority of deals provide product perks that should appear in your social post/ video/ photo. We will automatically ship these to you upon accepting a deal and we automate all of the cumbersome aspects of doing influencer deals. Details are below for deals in your feed:
	</p>
	<p style="font-size:14px; color:#000000; margin:0 0 12px 0;">

		<table border="0" cellpadding="20" cellspacing="0" width="900" style="font-size:14px;">
	    <tr>
	    	<td align="left" valign="middle" style="width: 100px;"><img src="https://dash.swayops.com{{Image}}" height="150"></td>
	    	<td align="left" valign="left">
		    	<b>Brand name:</b> {{Company}} <br/>
		    	<b>Campaign name:</b> {{Campaign}} <br/>
		    	<b>Your likely earnings based on your real followers/avg engagements:</b> ${{Payout}} <br/>
		    	<b>Product perks?:</b> {{Perks}} <br/>
		    	<b>Task description:</b> {{Task}} <br/>
	    	</td>
	    </tr>
		</table>
		
	</p>

	<p style="font-size:14px; color:#000000; margin:0 0 12px 0;">
		In order to access deals you simply need to sign up in our influencer app by <a href="https://inf.swayops.com/signup">Clicking Here</a> and hit the "Accept Endorsement" button inside of the deal you wish to participate in. Feel free to call or email me with any questions, we also have a full wiki on our website as well that explains how fast you get paid, how to calculate your average earnings, etc.
	</p>
	<p style="font-size:14px; color:#000000; margin:0 0 12px 0;">
		Very excited to work with you,<br/>
		~ Karlie M<br/>
	</p>
	<p style="font-size:14px; color:#000000; margin:0 0 12px 0;">
		<img src="http://swayops.com/swayEmailLogo.png" alt="" height="40" />
		<br/>
		Karlie@SwayOps.com | Office: 650-667-7929 | Address: 4461 Crossvine Dr, Prosper TX, 75078
	<br><br>
	Want to be taken off our influencer notification list?: <a href="https://dash.swayops.com/api/v1/optout/{{email}}">Click here</a> 
	</p>

</div>
`

const scrapDealTwo = `
<div>
	<p style="font-size:14px; color:#000000; margin:0 0 12px 0;">
		Hey again,
	</p>
	<p style="font-size:14px; color:#000000; margin:0 0 12px 0;">
		Hope you're doing well. Just wanted to reach out as we are excited to announce there are new Sways that requested your participation and I wanted to get your interest level:
	</p>
	<p style="font-size:14px; color:#000000; margin:0 0 12px 0;">

		<table border="0" cellpadding="20" cellspacing="0" width="900" style="font-size:14px;">
	    <tr>
	    	<td align="left" valign="middle" style="width: 100px;"><img src="https://dash.swayops.com{{Image}}" height="150"></td>
	    	<td align="left" valign="left">
		    	<b>Brand name:</b> {{Company}} <br/>
		    	<b>Campaign name:</b> {{Campaign}} <br/>
		    	<b>Budget available:</b> ${{Payout}} <br/>
		    	<b>Product perks?:</b> {{Perks}} <br/>
		    	<b>Task description:</b> {{Task}} <br/>
	    	</td>
	    </tr>
		</table>
		
	</p>
	<p style="font-size:14px; color:#000000; margin:0 0 12px 0;">
		In order to access these you simply need to sign up in our influencer app <a href="https://inf.swayops.com/signup">https://inf.swayops.com/signup</a> and hit the "Accept Endorsement" button. Feel free to call or email me with any questions.
	</p>
	<p style="font-size:14px; color:#000000; margin:0 0 12px 0;">
		All the best,<br/>
		~ Karlie M<br/>
	</p>
	<p style="font-size:14px; color:#000000; margin:0 0 12px 0;">
		<img src="http://swayops.com/swayEmailLogo.png" alt="" height="40" />
		<br/>
		Karlie@SwayOps.com | Office: 650-667-7929 | Address: 4461 Crossvine Dr, Prosper TX, 75078
	<br><br>
	Want to be taken off our influencer notification list?: <a href="https://dash.swayops.com/api/v1/optout/{{email}}">Click here</a> 
	</p>

</div>
`

var (
	ScrapFirstEmail = MustacheMust(scrapFirstEmail)
	ScrapDealOne    = MustacheMust(scrapDealOne)
	ScrapDealTwo    = MustacheMust(scrapDealTwo)
)
