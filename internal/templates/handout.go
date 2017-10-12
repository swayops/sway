package templates

const handoutTmpl = `
<div>
   <p>
      <img src="http://swayops.com/swayEmailLogo.png" alt="" height="40" /><br>
   </p>
   <p style="font-size:16px; color:#000000; margin:0 0 12px 0;">
      Hi {{Name}}! <br><br>

      Thank you for participating in this endorsement for {{Company}}. Please follow the deal details we have printed out below or you can find this info available to you within your Sway App at: https://inf.swayops.com/login <br><br>

    <b>Required items that must appear in your post:</b><br>
   <ul style="font-size:16px;">
      {{#Instructions}}
      <li>{{.}}</li>
      {{/Instructions}}
      <br>
   </ul>
   <br>
   Pleast DO NOT forget to hashtag #ad or #sponsored in order for your post to pass FTC compliance. Also if your campaign requires a link to be put in your caption or Instagram bio, please do this just before making your post to ensure you get paid for all clicks that occur.
    <br><br>
      <b>Deal Instructions:</b><br>
   <ul style="font-size:16px;">
      <li>{{Task}}</li>
      <br>
   </ul>

   <b>Choose 1 of the social networks on this list to post to:</b><br>
   <ul style="font-size:16px;">
      {{#Platforms}}
      <li>{{.}}</li>
      {{/Platforms}}
      <br>
   </ul>

   All the best,<br>
   ~ The Sway team <br>
   engage@swayops.com | Office: 650-667-7929 | Address: 4461 Crossvine Dr, Prosper TX, 75078
   </p>
</div>
`

var (
	Handout = MustacheMust(handoutTmpl)
)
