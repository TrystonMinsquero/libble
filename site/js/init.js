function init() {
	const userId = localStorage.getItem("userId");
	if (userId != null && window.location.href !== "/game") {
		window.location.href = "/game";
	} else if (window.location.href !== "/start") {
		window.location.href = "/start";
	}
}
