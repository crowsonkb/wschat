$(document).ready(function() {
	var ws = new WebSocket("ws://"+window.location.host+"/chat");
	ws.onmessage = function(event) {
		$("#messages").prepend($("<p>").text(event.data))
	};

	$("#input").keypress(function(event) {
		if (event.which == 13) {
			ws.send($("#input").val())
			$("#input").val("")
		}
	});
});
