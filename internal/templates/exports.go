package templates

const forecastTmpl = `
<html>
	<head>
		<link href="https://fonts.googleapis.com/css?family=Open+Sans" rel="stylesheet">
		<style>
			body {
				background-color: #fff;
				margin: 0px;
				font-family: 'Open Sans', sans-serif;
			}	
			.main {
				width:684px;
				height: 864px;
				margin: 0px;
				background-color: #fff;
			}
			.header {
				height: 34px;
				font-size: 14px;
				padding: 20px;
				background-color: #fff;
			}
			.footer {
				height: 15px;
				font-size: 12px;
				color: #848e92;
				padding: 15px;
				background-color: #fff;
			}
			.row {
				height: 160px;
				font-size: 14px;
				color: #000;
				padding: 15px;
				background-color: #f9f9f9;
				border-bottom: 1px #cecece solid;
				overflow:hidden;
			}
			.but-blue {
				background-color: #31aff5;
				border-radius: 5px;
				padding: 10px;
			}
			.forecast {
				background-color: #31aff5;
				color: #fff;
				padding: 20px;
			}
			.forecastItem {
				width: 30%;
				float: left;
				padding: 10px;
				font-weight: bold;
			}
			.label {
				padding: 10px;
				border-radius: 5px;
				background-color: #fff;
				color: #000;
				margin-top:10px;
			}
			.clearfix {
				clear:both;
				overflow: auto;
			}
			.infPic {
				width: 150px;
				height: 140px;
				border-radius: 5px;
				overflow: hidden;
				float: left;
			}
			.infDescription {
				margin-left: 20px;
				width: 300px;
				float: left;
			}
			h3 {
				margin:0px;
			}
			p {
				margin:0px;
			}
			.infStats {
				margin-left: 20px;
				width: 160px;
				float: left;
				margin-top: 30px;
				
			}
		</style>
		
	</head>
	<body>
		<div class="main">
		
			<div class="header">
				<img src="https://swayops.com/marketer/img/swayLogoBlack.png"/>
			</div>
			<div class="forecast clearfix">
				<div class="forecastItem">
					Budget:<br><div class="label" align="center">{{Budget}}</div>
				</div>
				<div class="forecastItem">
					Likely engagements:<br><div class="label" align="center">{{LikelyEngagements}}</div>
				</div>
				<div class="forecastItem">
					# of influencers:<br><div class="label" align="center">{{NumberOfInfluencers}}</div>
				</div>
			</div>
			
			{{#Influencers}}

			<div class="row clearfix" align="left">
				<div class="infPic">
					<img style="width: 100%;" src="{{ProfilePicture}}"/>
				</div>
				<div class="infDescription">
					<h3>{{Name}}</h3>
					<p>Gender: {{Gender}}</p>
					<p>Geo: {{Geo}}</p>
					<p>Categories: {{Categories}}</p>
					<p>
					{{#HasTwitter}}
						<img src="{{TwitterIcon}}"> <a href="https://twitter.com/{{TwitterUsername}}">@{{TwitterUsername}}</a>&nbsp
					{{/HasTwitter}}
					{{#HasYoutube}}
						<img src="{{YoutubeIcon}}"> <a href="https://www.youtube.com/channel/">@{{YoutubeUsername}}</a>&nbsp
					{{/HasYoutube}}
					{{#HasInsta}}
						<img src="{{InstaIcon}}"> <a href="https://www.instagram.com/{{InstaUsername}}">@{{InstaUsername}}</a>&nbsp
					{{/HasInsta}}
					{{#HasFacebook}}
						<img src="{{FacebookIcon}}"> <a href="https://www.facebook.com/{{FacebookUsername}}">@{{FacebookUsername}}</a>&nbsp
					{{/HasFacebook}}</p>
				</div>
				<div class="infStats">
					Followers: <b style="color:#31aff5;">{{StringFollowers}}</b> <br>
					Avg earnings: <b style="color:#31aff5;">{{MaxYield}}</b>
				</div>
			</div>

			{{/Influencers}}
			
			<div class="footer">
				<div align="center">&#169; 2017 Sway Ops LLC. - All rights reserved.</div>
			</div>
		
		</div>
	</body>
</html>
`

var ForecastExport = MustacheMust(forecastTmpl)

const CampaignReportTmp = `
<html>
	<head>
		<link href="https://fonts.googleapis.com/css?family=Open+Sans" rel="stylesheet">
		<style>
			body {
				background-color: #fff;
				margin: 0px;
				font-family: 'Open Sans', sans-serif;
			}	
			.main {
				width:684px;
				height: 864px;
				margin: 0px;
				background-color: #fff;
			}
			.header {
				height: 34px;
				font-size: 14px;
				padding: 20px;
				background-color: #fff;
			}
			.footer {
				height: 15px;
				font-size: 12px;
				color: #848e92;
				padding: 15px;
				background-color: #fff;
			}
			.row {
				height: 160px;
				font-size: 14px;
				color: #000;
				padding: 15px;
				background-color: #f9f9f9;
				border-bottom: 1px #cecece solid;
				overflow:hidden;
			}
			.but-blue {
				background-color: #31aff5;
				border-radius: 5px;
				padding: 10px;
			}
			.forecast {
				background-color: #31aff5;
				color: #fff;
				padding: 20px;
			}
			.forecastItem {
				width: 30%;
				float: left;
				padding: 10px;
				font-weight: bold;
			}
			.label {
				padding: 10px;
				border-radius: 5px;
				background-color: #fff;
				color: #000;
				margin-top:10px;
			}
			.clearfix {
				clear:both;
				overflow: auto;
			}
			.infPic {
				width: 150px;
				height: 140px;
				border-radius: 5px;
				overflow: hidden;
				float: left;
			}
			.infDescription {
				margin-left: 20px;
				width: 300px;
				float: left;
			}
			h3 {
				margin:0px;
			}
			p {
				margin:0px;
			}
			.infStats {
				margin-left: 20px;
				width: 160px;
				float: left;
				margin-top: 30px;
				
			}
		</style>
		
	</head>
	<body>
		<div class="main">
		
			<div class="header">
				<img src="https://swayops.com/marketer/img/swayLogoBlack.png"/>
			</div>
			<div class="forecast clearfix">
				<h3 align="center">{{Campaign Name}}</h3>
				<br/>
				<div class="forecastItem">
					Spent (USD):<br><div class="label" align="center">{{Spent}}</div>
				</div>
				<div class="forecastItem">
					Est Views:<br><div class="label" align="center">{{Views}}</div>
				</div>
				<div class="forecastItem">
					Engagements:<br><div class="label" align="center">{{Engagements}}</div>
				</div>
			</div>
			
			{{#Influencers}}

			<div class="row clearfix" align="left">
				<div class="infPic">
					<img style="width: 100%;" src="{{Picture}}"/>
				</div>
				<div class="infDescription">
					<h3>{{Name}}</h3>
					<p>Published: {{Date}}</p>
					<p>Post link: <a href="{{Link}}">Open</a></p>
					<p style="max-height: 80px;">Caption: {{Caption}}</p>
				</div>
				<div class="infStats">
					Est Views: <b style="color:#31aff5;">{{Views}}</b> <br>
					Likes: <b style="color:#31aff5;">{{Likes}}</b> <br>
					Comments: <b style="color:#31aff5;">{{Comments}}</b> <br>
					Shares: <b style="color:#31aff5;">{{Shares}}</b> <br>
					Clicks: <b style="color:#31aff5;">{{Clicks}}</b> <br>
				</div>
			</div>

			{{/Influencers}}
			
			<div class="footer">
				<div align="center">&#169; 2017 Sway Ops LLC. - All rights reserved.</div>
			</div>
		
		</div>
	</body>
</html>
`

var CampaignReportExport = MustacheMust(CampaignReportTmp)
