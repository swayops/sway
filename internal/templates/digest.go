package templates

const engineTmpl = `
<div>
	<p style="font-size:14px; color:#000000; margin:0;"><b>Start Time:</b> {{startTime}} </a></p>
	<p style="font-size:14px; color:#000000; margin:0;"><b>End Time:</b> {{endTime}} </a></p>
	<p style="font-size:14px; color:#000000; margin:0;"><b>Run Time:</b> {{runtime}} seconds</a></p><br>

	<p style="font-size:14px; color:#000000; margin:0;"><b>Updated Influencers:</b> {{updatedInf}} </a></p>
	<p style="font-size:14px; color:#000000; margin:0;"><b>New Deals Completed:</b> {{foundDeals}} </a></p>
	<p style="font-size:14px; color:#000000; margin:0;"><b>Signatures Completed:</b> {{sigsFound}} </a></p>
	<p style="font-size:14px; color:#000000; margin:0;"><b>Deals Emailed:</b> {{dealsEmailed}} </a></p>
	<p style="font-size:14px; color:#000000; margin:0;"><b>Scraps Emailed:</b> {{scrapsEmailed}} </a></p><br>

	<p style="font-size:14px; color:#000000; margin:0 0 12px 0;">
		<table border="0" cellpadding="20" cellspacing="0" width="600" style="font-size:14px;">
		<tr>
			<th align="left">Influencer:</th>
			<th align="left">Campaign:</th>
			<th align="left">Post:</th>
			<th align="left">Depleted:</th>
		</tr>
		{{#depletions}}
	    <tr>
	    	<td align="left" valign="middle">{{Influencer}}</td>
	    	<td align="left" valign="middle">{{Campaign}}</td>
	    	<td align="left" valign="middle">{{PostURL}}</td>
	    	<td align="left" valign="middle">${{Spent}}</td>
	    </tr>
	    {{/depletions}}
		</table>
	</p>
	<p style="font-size:14px; color:#000000; margin:0;"><b>Total Depleted:</b> ${{totalSpent}} </a></p><br>

	<p style="font-size:14px; color:#000000; margin:0;">Kind regards,</p>
	<p style="font-size:14px; color:#000000; margin:0;">The SwayOps Server.</p>
</div>
`

var EngineEmail = MustacheMust(engineTmpl)
