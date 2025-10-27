const form = document.getElementById("goodreads-user-form");

form.addEventListener("submit", function (e) {
	e.preventDefault();

	const submit_button = document.getElementById("submit-button");
	const user_id = document.getElementById("userId").value.trim();

	const url = "/scrape/" + user_id;
	console.log("Fetching ", url);
	fetch(url).then((res) => {
		if (res.ok) {
			res.json().then((json) => {
				console.log("Successfully scraped data:\n", json);
				// TODO: set some gobol value
				localStorage.setItem("userId", user_id);
				window.location.assign("/game");
			}).catch(
				console.log("Failed to get json"),
			);
		}
	}).catch(
		console.log("Failed to get json"),
	);
});
