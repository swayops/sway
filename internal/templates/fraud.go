package templates

const fraudTmpl = `
<div>
   <p style="font-size:16px; color:#000000; margin:0 0 12px 0;">
      ATTN: Admin <br><br>

   The deal explorer found the following fraud/ dishonesty for the post at <a href="{{URL}}">{{URL}}</a> for Campaign {{CampaignID}} and Influencer {{InfluencerID}}<br>
   <ul style="font-size:16px; color:#000000; margin:0 0 12px 0;">
      {{#Reasons}}
      <li>{{.}}</li>
      {{/Reasons}}
      <br>
   </ul>

	<a href="https://dash.swayops.com/api/v1/{{AllowURL}}" style="background-color:#04B431;border:1px solid #04B431;border-radius:3px;color:#ffffff;display:inline-block;font-family:sans-serif;font-size:16px;line-height:44px;text-align:center;text-decoration:none;width:150px;">Allow</a>
	<a href="https://dash.swayops.com/api/v1/{{StrikeURL}}" style="background-color:#EB7035;border:1px solid #EB7035;border-radius:3px;color:#ffffff;display:inline-block;font-family:sans-serif;font-size:16px;line-height:44px;text-align:center;text-decoration:none;width:150px;">Strike</a>
	<a href="https://dash.swayops.com/api/v1/{{BanURL}}" style="background-color:#FF0040;border:1px solid #FF0040;border-radius:3px;color:#ffffff;display:inline-block;font-family:sans-serif;font-size:16px;line-height:44px;text-align:center;text-decoration:none;width:150px;">Ban</a>

	<br><br>

   All the best,<br>
   ~ The Sway Server <br>
   </p>

</div>
`

var FraudEmail = MustacheMust(fraudTmpl)
