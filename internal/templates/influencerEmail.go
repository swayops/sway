package templates

const infEmailTmpl = `
<div>
	<p style="font-size:14px; color:#000000; margin:0 0 12px 0;">
		Hi {{Name}},
	</p>
	<p style="font-size:14px; color:#000000; margin:0 0 12px 0;">
		This is a system notification. New Sways campaigns have requested your participation:
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
		~ The Sway team<br/>
	</p>
	<p style="font-size:14px; color:#000000; margin:0 0 12px 0;">
		<img src="http://swayops.com/swayEmailLogo.png" alt="" height="40" />
		<br/>
		engage@SwayOps.com | Office: 650-667-7929 | Address: 4461 Crossvine Dr, Prosper TX, 75078
	</p>

</div>
`

const infCmpEmail = `
<div>
	<p style="font-size:14px; color:#000000; margin:0 0 12px 0;">
		Hi {{Name}},
	</p>
	<p style="font-size:14px; color:#000000; margin:0 0 12px 0;">
		This is a system notification. We are excited to announce there is a new Sway that requested your participation:
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
		~ The Sway team<br/>
	</p>
	<p style="font-size:14px; color:#000000; margin:0 0 12px 0;">
		<img src="http://swayops.com/swayEmailLogo.png" alt="" height="40" />
		<br/>
		engage@SwayOps.com | Office: 650-667-7929 | Address: 4461 Crossvine Dr, Prosper TX, 75078
	</p>
</div>
`

const headsUpEmail = `
<div>
	<p style="font-size:14px; color:#000000; margin:0 0 12px 0;">
		Hey {{Name}},
	</p>
	<p style="font-size:14px; color:#000000; margin:0 0 12px 0;">
		This is a system notification to let you know that you only have 7 days left to complete your Sway deal for {{Company}}. After the 7 days, we will unfortunately be forced to retract the deal from you!
	</p>
	<p style="font-size:14px; color:#000000; margin:0 0 12px 0;">
		If you would like to access the deal requirements, please visit <a href="https://inf.swayops.com/login">https://inf.swayops.com/login</a> <br/> Feel free to call or email me or our team with any questions.
	</p>
	<p style="font-size:14px; color:#000000; margin:0 0 12px 0;">
		All the best,<br/>
		~ The Sway team<br/>
	</p>
	<p style="font-size:14px; color:#000000; margin:0 0 12px 0;">
		<img src="http://swayops.com/swayEmailLogo.png" alt="" height="40" />
		<br/>
		engageSwayOps.com | Office: 650-667-7929 | Address: 4461 Crossvine Dr, Prosper TX, 75078
	</p>
</div>
`

const dealPostAlert = `
<div>
	<p style="font-size:14px; color:#000000; margin:0 0 12px 0;">
		Hey {{Name}},
	</p>
	<p style="font-size:14px; color:#000000; margin:0 0 12px 0;">
		Checking in to see the status of your post for {{Company}}. Please let us know when you plan on posting and whether you have any questions or concerns regarding the post that needs to be made.
	</p>
	<p style="font-size:14px; color:#000000; margin:0 0 12px 0;">
		If you would like to access the deal requirements, please visit <a href="https://inf.swayops.com/login">https://inf.swayops.com/login</a> <br/> Feel free to call or email me or our team with any questions.
	</p>
	<p style="font-size:14px; color:#000000; margin:0 0 12px 0;">
		All the best,<br/>
		~ The Sway team<br/>
	</p>
	<p style="font-size:14px; color:#000000; margin:0 0 12px 0;">
		<img src="http://swayops.com/swayEmailLogo.png" alt="" height="40" />
		<br/>
		engageSwayOps.com | Office: 650-667-7929 | Address: 4461 Crossvine Dr, Prosper TX, 75078
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
		This is considered a violation of the terms you signed upon accepting this deal. Your Sway influencer score has been lowered because of this and it will be instantly visible to all partners and public rank lists. If your receiving this message in error, please contact us immedietly to avoid negative remarks on your profile. Future infractions are likely to yeild permanent bans from the marketplace and partner marketplaces in the influencer space.
	</p>
	<p style="font-size:14px; color:#000000; margin:0 0 12px 0;">
		Regards,<br/>
		~ The Sway system<br/>
	</p>
	<p style="font-size:14px; color:#000000; margin:0 0 12px 0;">
		<img src="http://swayops.com/swayEmailLogo.png" alt="" height="40" />
		<br/>
		engage@SwayOps.com | Office: 650-667-7929 | Address: 4461 Crossvine Dr, Prosper TX, 75078
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
		Feel free to call or email us with any questions.
	</p>
	<p style="font-size:14px; color:#000000; margin:0 0 12px 0;">
		Regards,<br/>
		~ The Sway notification system<br/>
	</p>
	<p style="font-size:14px; color:#000000; margin:0 0 12px 0;">
		<img src="http://swayops.com/swayEmailLogo.png" alt="" height="40" />
		<br/>
		engage@SwayOps.com | Office: 650-667-7929 | Address: 4461 Crossvine Dr, Prosper TX, 75078
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
		Feel free to call or email us with any questions.
	</p>
	<p style="font-size:14px; color:#000000; margin:0 0 12px 0;">
		Regards,<br/>
		~ The Sway notifications system<br/>
	</p>
	<p style="font-size:14px; color:#000000; margin:0 0 12px 0;">
		<img src="http://swayops.com/swayEmailLogo.png" alt="" height="40" />
		<br/>
		engage@SwayOps.com | Office: 650-667-7929 | Address: 4461 Crossvine Dr, Prosper TX, 75078
	</p>
</div>
`

const pickedUpTmpl = `
<div>
	<p style="font-size:14px; color:#000000; margin:0 0 12px 0;">
		Hi {{Name}}!
	</p>
	<p style="font-size:14px; color:#000000; margin:0 0 12px 0;">
		Your deal for {{Company}} has been noticed by Sway! However, the post is still awaiting admin approval so please allow up to 24 hours for the post to show up in your Sway Stats on the <a href="https://inf.swayops.com/login">Influencer Dashboard</a>. <br><br>

		We will notify you via email as well once the post has been approved by Sway.
	</p>
	<p style="font-size:14px; color:#000000; margin:0 0 12px 0;">
		Feel free to call or email us with any questions.
	</p>
	<p style="font-size:14px; color:#000000; margin:0 0 12px 0;">
		Regards,<br/>
		~ The Sway team<br/>
	</p>
	<p style="font-size:14px; color:#000000; margin:0 0 12px 0;">
		<img src="http://swayops.com/swayEmailLogo.png" alt="" height="40" />
		<br/>
		engage@SwayOps.com | Office: 650-667-7929 | Address: 4461 Crossvine Dr, Prosper TX, 75078
	</p>
</div>
`

const campaignStatusEmail = `
<div>
	<p style="font-size:14px; color:#000000; margin:0 0 12px 0;">
		Hey {{Name}},
	</p>
	<p style="font-size:14px; color:#000000; margin:0 0 12px 0;">
		We regret to inform you that the deal you had accepted for {{Company}} is no longer available due to the campaign changing it's requirements. This can occur for several reasons outside of our control but it is most likely due to the brand changing requirements or moving budget to other campaigns they have running.
	</p>
	<p style="font-size:14px; color:#000000; margin:0 0 12px 0;">
		If you would like to access more deals, please visit <a href="https://inf.swayops.com/login">https://inf.swayops.com/login</a> .
	</p>
	<p style="font-size:14px; color:#000000; margin:0 0 12px 0;">
		Regards,<br/>
		~ The Sway team<br/>
	</p>
	<p style="font-size:14px; color:#000000; margin:0 0 12px 0;">
		<img src="http://swayops.com/swayEmailLogo.png" alt="" height="40" />
		<br/>
		engage@SwayOps.com | Office: 650-667-7929 | Address: 4461 Crossvine Dr, Prosper TX, 75078
	</p>
</div>
`

const dealRejectionEmail = `
<div>
	<p style="font-size:14px; color:#000000; margin:0 0 12px 0;">
		Hi {{Name}},
	</p>
	<p style="font-size:14px; color:#000000; margin:0 0 12px 0;">
		Your most recent deal post ( {{url}} ) is missing a required item. Unfortunately our engine can't pickup your completed deal because of this. Please double check that you included the <b>{{reason}}</b> in your post and the system will automatically authorize your post.	</p>
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
		Our engine detected that one or more of your social media profiles linked with your Sway account is now private and/or inaccessible. Until your profile is made public, we cannot offer you any Sway deals, track your stats or detect when you complete deals. Let us know if you have any questions. Happy to help 	<p style="font-size:14px; color:#000000; margin:0 0 12px 0;">
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

const perkMailEmail = `
<div>
	<p style="font-size:14px; color:#000000; margin:0 0 12px 0;">
		Hi {{Name}}!
	</p>
	<p style="font-size:14px; color:#000000; margin:0 0 12px 0;">
		We are emailing to inform you that we have just sent out your perk for the deal you have accepted for {{Company}}. Please allow atleast 5-7 business days for the product to arrive.
	</p>
	<p style="font-size:14px; color:#000000; margin:0 0 12px 0;">
		Feel free to call or email us with any questions.
	</p>
	<p style="font-size:14px; color:#000000; margin:0 0 12px 0;">
		Regards,<br/>
		~ The Sway team<br/>
	</p>
	<p style="font-size:14px; color:#000000; margin:0 0 12px 0;">
		<img src="http://swayops.com/swayEmailLogo.png" alt="" height="40" />
		<br/>
		engage@SwayOps.com | Office: 650-667-7929 | Address: 4461 Crossvine Dr, Prosper TX, 75078
	</p>
</div>
`

const dealInstructionsEmail = `
<div>
	<p style="font-size:14px; color:#000000; margin:0 0 12px 0;">
		Hi {{Name}},
	</p>

	<p style="font-size:14px; color:#000000; margin:0 0 12px 0;">
		Thank you for accepting the deal for {{Campaign}}! The brand is excited to be working with you. <br>
   <br>Pleast DO NOT forget to hashtag #ad or #sponsored in order for your post to pass FTC compliance. Also if your campaign requires a link to be put in your caption or Instagram bio, please do this just before making your post to ensure you get paid for all clicks that occur.
    <br><br>Here are some instructions on how to complete this deal:
	</p>

	<p style="font-size:14px; color:#000000; margin:0 0 12px 0;">
		<table border="0" cellpadding="20" cellspacing="0" width="900" style="font-size:14px;">
	    <tr>
	    	<td align="left" valign="middle" style="width: 100px;"><img src="https://dash.swayops.com{{Image}}" height="150"></td>
	    	<td align="left" valign="left">
		    	<b>Campaign name:</b> {{Campaign}} <br/>
		    	<b>Task description:</b> {{Task}} <br/>
		    	<b>Please post to ONLY one of the following networks:</b> {{Networks}} <br/>

		    	{{#HasPerks}}
					<br/>
			    	<b>Perks:</b> {{Perks}} <br/>
		 		{{/HasPerks}}

		    	{{#HasAddress}}
		    		<br/>
			    	<b>Will be mailed to: </b> {{Address}} <br/>
		 		{{/HasAddress}}

		    	{{#HasCoupon}}
		    		<br/>
			 		<b>Coupon Code:</b> {{CouponCode}} <br/>
			 		<b>Instructions:</b> {{Instructions}} <br/>
		 		{{/HasCoupon}}

		    	{{#HasSchedule}}
					<br/>
			 		Must be posted between <b>{{StartTime}}</b> and <b>{{EndTime}}</b> <br/>
		 		{{/HasSchedule}}

				{{^HasSchedule}}
		    		<br/>
			    	<b>Days to complete:</b> {{Timeout}}<br/>
				{{/HasSchedule}}

		 		<br/>
		    	<b>Put this link in your bio/caption:</b> {{Link}}<br/>
				<b>Hashtags to do:</b> {{Tags}}<br/>
				<b>Mentions to do:</b> {{Mentions}}<br/>
		 		<br/>
	    	</td>
	    </tr>
		</table>

	</p>

	<p style="font-size:14px; color:#000000; margin:0 0 12px 0;">
		Let us know if you have any questions! <br/><br/>
		Regards,<br/>
		~ The Sway notification system<br/>
	</p>
	<p style="font-size:14px; color:#000000; margin:0 0 12px 0;">
		<img src="http://swayops.com/swayEmailLogo.png" alt="" height="40" />
		<br/>
		Engage@SwayOps.com | Office: 650-667-7929 | Address: 4461 Crossvine Dr, Prosper TX, 75078
	</p>
</div>
`

const submissionInstructionsEmail = `
<div>
	<p style="font-size:14px; color:#000000; margin:0 0 12px 0;">
		Hi {{Name}},
	</p>

	<p style="font-size:14px; color:#000000; margin:0 0 12px 0;">
		Thank you for accepting the deal for {{Campaign}}! The brand is excited to be working with you. Here are some instructions on how to complete this deal:
	</p>

	<p style="font-size:14px; color:#000000; margin:0 0 12px 0;">
		<table border="0" cellpadding="20" cellspacing="0" width="900" style="font-size:14px;">
	    <tr>
	    	<td align="left" valign="middle" style="width: 100px;"><img src="https://dash.swayops.com{{Image}}" height="150"></td>
	    	<td align="left" valign="left">
		    	<b>Campaign name:</b> {{Campaign}} <br/>
		    	<b>Task description:</b> {{Task}} <br/>
		    	<b>Instructions:</b> <br/>
		    	1) Draft your post's caption and/or image and submit via the Influencer App <br/>
		    	2) Await advertiser's approval of your drafted post (you will be notified via email) <br/>
		    	3) Once approved, you may go ahead and post the approved draft to your social handle<br/>

				<br/>
		    	<b>Please post to ONLY one of the following networks:</b> {{Networks}} <br/>

		    	{{#HasPerks}}
					<br/>
			    	<b>Perks:</b> {{Perks}} <br/>
		 		{{/HasPerks}}

		    	{{#HasAddress}}
		    		<br/>
			    	<b>Will be mailed to: </b> {{Address}} <br/>
		 		{{/HasAddress}}

		    	{{#HasCoupon}}
		    		<br/>
			 		<b>Coupon Code:</b> {{CouponCode}} <br/>
			 		<b>Instructions:</b> {{Instructions}} <br/>
		 		{{/HasCoupon}}

		    	{{#HasSchedule}}
					<br/>
			 		Must be posted between <b>{{StartTime}}</b> and <b>{{EndTime}}</b> <br/>
		 		{{/HasSchedule}}

				{{^HasSchedule}}
			    	<b>Days to complete:</b> {{Timeout}}<br/>
				{{/HasSchedule}}

		 		<br/>
		    	<b>Put this link in your bio/caption:</b> {{Link}}<br/>
				<b>Hashtags to do:</b> {{Tags}}<br/>
				<b>Mentions to do:</b> {{Mentions}}<br/>
		 		<br/>
	    	</td>
	    </tr>
		</table>

	</p>

	<p style="font-size:14px; color:#000000; margin:0 0 12px 0;">
		Let us know if you have any questions! <br/><br/>
		Regards,<br/>
		~ The Sway team<br/>
	</p>
	<p style="font-size:14px; color:#000000; margin:0 0 12px 0;">
		<img src="http://swayops.com/swayEmailLogo.png" alt="" height="40" />
		<br/>
		Engage@SwayOps.com | Office: 650-667-7929 | Address: 4461 Crossvine Dr, Prosper TX, 75078
	</p>
</div>
`

const submissionApprovedEmail = `
<div>
	<p style="font-size:14px; color:#000000; margin:0 0 12px 0;">
		Hi {{Name}},
	</p>

	<p style="font-size:14px; color:#000000; margin:0 0 12px 0;">
		Congratulations!
	</p>
	<p style="font-size:14px; color:#000000; margin:0 0 12px 0;">
		Your post submission for {{Company}} has just been approved! You may now go ahead and make the post. Remember to keep the the same caption and/or media as your approved submission otherwise your post will not be approved. Please allow up to 24 hours after making the post for Sway to notice it!
	</p>
	<p style="font-size:14px; color:#000000; margin:0 0 12px 0;">
		Feel free to call or email me with any questions.
	</p>
	<p style="font-size:14px; color:#000000; margin:0 0 12px 0;">
		Regards,<br/>
		~ The Sway notification system<br/>
	</p>
	<p style="font-size:14px; color:#000000; margin:0 0 12px 0;">
		<img src="http://swayops.com/swayEmailLogo.png" alt="" height="40" />
		<br/>
		Engage@SwayOps.com | Office: 650-667-7929 | Address: 4461 Crossvine Dr, Prosper TX, 75078
	</p>
</div>
`

const auditEmailTmpl = `
<div>
	<p style="font-size:14px; color:#000000; margin:0 0 12px 0;">
		Hi {{Name}},
	</p>

	<p style="font-size:14px; color:#000000; margin:0 0 12px 0;">
		Congratulations!
	</p>
	<p style="font-size:14px; color:#000000; margin:0 0 12px 0;">
		I wanted to reach out and let you know that your account has just passed our internal audit. Your account with Sway is now activated and any deals you were notified about will now appear in your feed if they are still available. Let us know if you have any questions and thank you so much for signing up with Sway. We know influencers are extremely busy, our team will only reach out when we have new deals or something is critically needed. 	</p>
	<p style="font-size:14px; color:#000000; margin:0 0 12px 0;">
		Feel free to call or email me with any questions.
	</p>
	<p style="font-size:14px; color:#000000; margin:0 0 12px 0;">
		Regards,<br/>
		~ The Sway notification system<br/>
	</p>
	<p style="font-size:14px; color:#000000; margin:0 0 12px 0;">
		<img src="http://swayops.com/swayEmailLogo.png" alt="" height="40" />
		<br/>
		Engage@SwayOps.com | Office: 650-667-7929 | Address: 4461 Crossvine Dr, Prosper TX, 75078
	</p>
</div>
`

var (
	AuditEmail                  = MustacheMust(auditEmailTmpl)
	InfluencerEmail             = MustacheMust(infEmailTmpl)
	InfluencerCmpEmail          = MustacheMust(infCmpEmail)
	InfluencerHeadsUpEmail      = MustacheMust(headsUpEmail)
	DealPostAlert               = MustacheMust(dealPostAlert)
	InfluencerTimeoutEmail      = MustacheMust(timeOutEmail)
	CheckEmail                  = MustacheMust(checkTmpl)
	DealCompletionEmail         = MustacheMust(completionTmpl)
	PickedUpEmail               = MustacheMust(pickedUpTmpl)
	CampaignStatusEmail         = MustacheMust(campaignStatusEmail)
	DealRejectionEmail          = MustacheMust(dealRejectionEmail)
	PrivateEmail                = MustacheMust(privateEmail)
	PerkMailEmail               = MustacheMust(perkMailEmail)
	DealInstructionsEmail       = MustacheMust(dealInstructionsEmail)
	SubmissionInstructionsEmail = MustacheMust(submissionInstructionsEmail)
	SubmissionApprovedEmail     = MustacheMust(submissionApprovedEmail)
)
