package templates

const engineTmpl = `
<div>
	<p style="font-size:14px; color:#000000; margin:0;"><b>Start Time:</b> {{startTime}} </a></p>
	<p style="font-size:14px; color:#000000; margin:0;"><b>End Time:</b> {{endTime}} </a></p>
	<p style="font-size:14px; color:#000000; margin:0;"><b>Run Time:</b> {{runtime}} seconds</a></p><br>

	<p style="font-size:14px; color:#000000; margin:0;"><b>Updated Influencers:</b> {{updatedInf}} </a></p>
	<p style="font-size:14px; color:#000000; margin:0;"><b>New Deals Completed:</b> {{foundDeals}} </a></p>
	<p style="font-size:14px; color:#000000; margin:0;"><b>Budget Depleted:</b> ${{totalDepleted}} </a></p>
	<p style="font-size:14px; color:#000000; margin:0;"><b>Signatures Completed:</b> {{sigsFound}} </a></p>
	<p style="font-size:14px; color:#000000; margin:0;"><b>Deals Emailed:</b> {{dealsEmailed}} </a></p><br>
	<p style="font-size:14px; color:#000000; margin:0;"><b>Scraps Emailed:</b> {{scrapsEmailed}} </a></p><br>

	<p style="font-size:14px; color:#000000; margin:0;">Kind regards,</p>
	<p style="font-size:14px; color:#000000; margin:0;">The SwayOps Server.</p>
</div>
`

var EngineEmail = MustacheMust(engineTmpl)
