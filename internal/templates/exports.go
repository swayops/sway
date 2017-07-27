package templates

const forecastTmpl = `
<html>
	<head>
		<link href="https://fonts.googleapis.com/css?family=Open+Sans" rel="stylesheet">
		<style>
			body {
				background-color: #000;
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
				height: 110px;
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
				overflow: auto;
			}
			.infPic {
				width: 150px;
				height: 100px;
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

			<div class="row" align="left">
				<div class="infPic">
					<img src="https://dash.swayops.com/static/img/hdr-sign-bg.png"/>
				</div>
				<div class="infDescription">
					<h3>{{Name}}</h3>
					<p>Gender: {{Gender}}</p>
					<p>Geo: {{Geo}}</p>
					<p>Categories: {{Categories}}</p>
					<p>{{SocialHandles}}</p>
				</div>
				<div class="infStats">
					Followers: <b style="color:#31aff5;">{{Followers}}</b> <br>
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
