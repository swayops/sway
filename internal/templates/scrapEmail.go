package templates

const scrapFirstEmail = `
<div>
	<p style="font-size:14px; color:#000000; margin:0 0 12px 0;">
		Hey {{Name}},
	</p>
	<p style="font-size:14px; color:#000000; margin:0 0 12px 0;">
		Our company makes software that connects social media influencers with brands.
	</p>
	<p style="font-size:14px; color:#000000; margin:0 0 12px 0;">
		I thought I would ping you because we have a few big advertisers coming through in the next 30 days and our software picked you up as a candidate for them. Just wanted to see if this was something that would interest you going forward. 
	</p>
	<p style="font-size:14px; color:#000000; margin:0 0 12px 0;">
		We generally make influencers double the revenue they would normally bring in through your avg social post. You don't need to go back and forth over email for every opportunity, we simply show it via a feed in our mobile app. We handle payments, 1099's, shipping free products to you, and all the non-fun stuff so you can focus on your fans and developing your social brand.
	</p>
	<p style="font-size:14px; color:#000000; margin:0 0 12px 0;">
		If this sounds like something that would interest you please let us know. You can see more info about how our app works at: http://SwayOps.com/influencer/ <br/>
		and if I don't hear from you I will ping you over email when the next brand requests you :)

		Hope to work together soon,<br/>
		~ Karlie M<br/>
	</p>
	<p style="font-size:14px; color:#000000; margin:0 0 12px 0;">
		<img src="http://swayops.com/swayEmailLogo.png" alt="" height="40" />
		<br/>
		Karlie@SwayOps.com | Office: 650-667-7929 | Address: 4461 Crossvine Dr, Prosper TX, 75078
	</p>

</div>
`

const scrapDealOne = `
<div>
	<p style="font-size:14px; color:#000000; margin:0 0 12px 0;">
		Hey {{Name}},
	</p>
	<p style="font-size:14px; color:#000000; margin:0 0 12px 0;">
		Just wanted to follow up with a few deals that would be available for you based on my previous email. These are deals you fit based on your fan analytics, follower counts, avg engagement rates, etc. As your personal brand and Sway reputation grows you will begin to see much larger opportunities inside of the influencer portal. Here are a few options for you:
	</p>
	<p style="font-size:14px; color:#000000; margin:0 0 12px 0;">
		<table border="0" cellpadding="20" cellspacing="0" width="600" style="font-size:14px;">
		<tr>
			<th align="left"></th>
			<th align="left">Company:</th>
			<th align="left">Campaign name:</th>
		</tr>
	    <tr>
	    	<td align="left" valign="middle"><img src="https://dash.swayops.com{{Image}}" height="50"></td>
	    	<td align="left" valign="middle">{{Company}}</td>
	    	<td align="left" valign="middle">{{Campaign}}</td>
	    </tr>
		</table>
	</p>

	<p style="font-size:14px; color:#000000; margin:0 0 12px 0;">
		In order to access these you simply need to sign up in our influencer app <a href="https://inf.swayops.com/signup">https://inf.swayops.com/signup</a> and hit the "Accept Endorsement" button. Feel free to call or email me with any questions, we also have a full wiki on our website as well.
	</p>
	<p style="font-size:14px; color:#000000; margin:0 0 12px 0;">
		Very excited to work with you,<br/>
		~ Karlie M<br/>
	</p>
	<p style="font-size:14px; color:#000000; margin:0 0 12px 0;">
		<img src="http://swayops.com/swayEmailLogo.png" alt="" height="40" />
		<br/>
		Karlie@SwayOps.com | Office: 650-667-7929 | Address: 4461 Crossvine Dr, Prosper TX, 75078
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
		<table border="0" cellpadding="20" cellspacing="0" width="600" style="font-size:14px;">
		<tr>
			<th align="left"></th>
			<th align="left">Company:</th>
			<th align="left">Campaign name:</th>
		</tr>
	    <tr>
	    	<td align="left" valign="middle"><img src="https://dash.swayops.com{{Image}}" height="50"></td>
	    	<td align="left" valign="middle">{{Company}}</td>
	    	<td align="left" valign="middle">{{Campaign}}</td>
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
	</p>
</div>
`

var (
	ScrapFirstEmail = MustacheMust(scrapFirstEmail)
	ScrapDealOne    = MustacheMust(scrapDealOne)
	ScrapDealTwo    = MustacheMust(scrapDealTwo)
)
