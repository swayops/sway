package templates

const forecastTmpl = `<!DOCTYPE html>
<html>
	<head>
		<!-- Loading Flat UI -->
		<link href="https://dash.swayops.com/static/css/flat-ui.min.css" rel="stylesheet">
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
				overflow: hidden;
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
				float: left;
				margin-top: 42px;

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
						<i href="http://www.twitter.com" title="Twitter" class="fui-twitter"></i> @{{TwitterUsername}}&nbsp
					{{/HasTwitter}}
					{{#HasYoutube}}
						<i title="YouTube" class="fui-youtube"></i> @{{YoutubeUsername}}&nbsp
					{{/HasYoutube}}
					{{#HasInsta}}
						<i title="Instagram" class="fui-instagram"></i> @{{InstaUsername}}&nbsp
					{{/HasInsta}}
					{{#HasFacebook}}
						<i title="Facebook" class="fui-facebook"></i> @{{FacebookUsername}}&nbsp
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
