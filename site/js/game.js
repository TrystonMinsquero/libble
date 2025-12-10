let books = [];
let allQuotes = [];
let currentQuote = null;
let usedQuoteIds = new Set();
let attemptCount = 0;
const MAX_ATTEMPTS = 5;
let previousGuesses = [];
let hintsUsed = [];

// Load data from JSON files
async function loadData() {
    try {
        const [booksResponse, quotesResponse] = await Promise.all([
            fetch("./books.json"),
            fetch("./quotes.json"),
        ]);

        const booksData = await booksResponse.json();
        const quotesData = await quotesResponse.json();

        // Process books - only include books with valid dates_read
        books = booksData
            .filter((book) =>
                book.dates_read && book.dates_read.length > 0 &&
                book.dates_read[0] !== "not set"
            )
            .map((book) => {
                // Get the most recent read date
                const mostRecentDate = book.dates_read[
                    book.dates_read.length - 1
                ];
                return {
                    book_id: book.book_id,
                    title: book.raw_title.split("\n")[0]
                        .trim(),
                    author: book.author.split(",").reverse()
                        .join(" ").trim(),
                    date_read: mostRecentDate,
                };
            });

        // Flatten quotes array and match with books
        allQuotes = quotesData.flat().map((quote) => {
            const book = books.find((b) => b.book_id === quote.book_id);
            return {
                quote_id: quote.quote_id,
                text: quote.text,
                book_id: quote.book_id,
                book: book,
            };
        }).filter((q) => q.book); // Only keep quotes with matching books

        console.log(
            `Loaded ${books.length} books and ${allQuotes.length} quotes`,
        );

        // Load used quotes from localStorage
        const storedUsed = localStorage.getItem("usedQuoteIds");
        if (storedUsed) {
            usedQuoteIds = new Set(JSON.parse(storedUsed));
        }

        // Reset if all quotes have been used
        if (usedQuoteIds.size >= allQuotes.length) {
            usedQuoteIds.clear();
            localStorage.removeItem("usedQuoteIds");
        }

        // Select today's quote
        selectDailyQuote();
    } catch (error) {
        console.error("Error loading data:", error);
        // Fallback data
        books = [
            { title: "Pride and Prejudice", author: "Jane Austen" },
            { title: "1984", author: "George Orwell" },
        ];
        currentQuote = {
            text: "It is a truth universally acknowledged...",
            book: books[0],
        };
        document.getElementById("quote").textContent = currentQuote.text;
    }
}

function selectDailyQuote() {
    // Get today's date as a seed
    const today = new Date();
    const daysSinceEpoch = Math.floor(
        today.getTime() / (1000 * 60 * 60 * 24),
    );

    // Get available quotes (not used yet)
    const availableQuotes = allQuotes.filter((q) =>
        !usedQuoteIds.has(q.quote_id)
    );

    if (availableQuotes.length === 0) {
        // All quotes used, reset
        usedQuoteIds.clear();
        localStorage.removeItem("usedQuoteIds");
        return selectDailyQuote();
    }

    // Use day as seed for consistent daily selection
    const index = daysSinceEpoch % availableQuotes.length;
    currentQuote = availableQuotes[index];

    // Mark as used
    usedQuoteIds.add(currentQuote.quote_id);
    localStorage.setItem("usedQuoteIds", JSON.stringify([...usedQuoteIds]));

    // Reset attempt count for new quote
    attemptCount = 0;
    previousGuesses = [];
    hintsUsed = [];

    // Reset hint buttons
    document.getElementById("timeHintBtn").disabled = false;
    document.getElementById("hintDisplay").textContent = "";
    document.getElementById("hintDisplay").style.display = "none";

    // Display the quote
    document.getElementById("quote").textContent = currentQuote.text;
}

document.getElementById("guessForm").addEventListener("submit", function (e) {
    e.preventDefault();

    if (!currentQuote) return;

    const title = document.getElementById("title").value.trim()
        .toLowerCase();
    const author = document.getElementById("author").value.trim()
        .toLowerCase();
    const feedbackBox = document.getElementById("feedbackBox");

    const correctTitle = currentQuote.book.title.toLowerCase();
    const correctAuthor = currentQuote.book.author.toLowerCase();

    // Check if the book exists in the library
    const isValidBook = books.some((book) =>
        book.title.toLowerCase() === title &&
        book.author.toLowerCase() === author
    );

    if (!isValidBook) {
        feedbackBox.textContent = `⚠️ That book is not in your library!`;
        feedbackBox.className = "feedback warning";
        return;
    }

    // Check if this guess was already made
    const guessKey = `${title}|${author}`;
    if (previousGuesses.includes(guessKey)) {
        feedbackBox.textContent = `⚠️ You already tried that guess!`;
        feedbackBox.className = "feedback warning";
        return;
    }

    previousGuesses.push(guessKey);
    attemptCount++;

    if (title === correctTitle && author === correctAuthor) {
        feedbackBox.textContent =
            `🎉 Correct! You got it in ${attemptCount} attempt${
                attemptCount === 1 ? "" : "s"
            }!`;
        feedbackBox.className = "feedback success";
        disableInputs();
    } else if (attemptCount >= MAX_ATTEMPTS) {
        feedbackBox.textContent =
            `❌ Failed! The answer was "${currentQuote.book.title}" by ${currentQuote.book.author}`;
        feedbackBox.className = "feedback error";
        disableInputs();
    } else if (author === correctAuthor && title !== correctTitle) {
        feedbackBox.textContent =
            `✍️ You got the author! Now guess the title. (${attemptCount}/${MAX_ATTEMPTS})`;
        feedbackBox.className = "feedback partial";
    } else if (title === correctTitle && author !== correctAuthor) {
        feedbackBox.textContent =
            `📚 You got the title! Now guess the author. (${attemptCount}/${MAX_ATTEMPTS})`;
        feedbackBox.className = "feedback partial";
    } else {
        feedbackBox.textContent =
            `❌ Nope! Try again. (${attemptCount}/${MAX_ATTEMPTS})`;
        feedbackBox.className = "feedback error";
    }
});

function disableInputs() {
    document.getElementById("title").disabled = true;
    document.getElementById("author").disabled = true;
    document.querySelector(".submit-btn").disabled = true;
    document.getElementById("timeHintBtn").disabled = true;
}

// Time hint functionality
document.getElementById("timeHintBtn").addEventListener("click", function () {
    if (!currentQuote || hintsUsed.includes("time")) return;

    const dateRead = currentQuote.book.date_read;
    const hintDisplay = document.getElementById("hintDisplay");

    // Parse the date (format: "MMM DD, YYYY" like "Feb 25, 2014")
    const date = new Date(dateRead);
    const year = date.getFullYear();

    // Create a 2-year range around the read date
    const startYear = year - 1;
    const endYear = year + 1;

    hintDisplay.textContent =
        `📅 You read this book between ${startYear} and ${endYear}`;
    hintDisplay.style.display = "block";

    hintsUsed.push("time");
    this.disabled = true;
});

function fuzzyMatch(query, text) {
    query = query.toLowerCase();
    text = text.toLowerCase();

    // Check if text starts with query
    if (text.startsWith(query)) {
        return 1.0;
    }

    // Check if any word in text starts with query
    const words = text.split(/\s+/);
    for (let word of words) {
        if (word.startsWith(query)) {
            return 0.9;
        }
    }

    // Check if query is contained in text
    if (text.includes(query)) {
        return 0.8;
    }

    // Fall back to Levenshtein distance
    const cleanQuery = query.replace(/[^a-z0-9]/g, "");
    const cleanText = text.replace(/[^a-z0-9]/g, "");

    const matrix = [];
    for (let i = 0; i <= cleanText.length; i++) matrix[i] = [i];
    for (let j = 0; j <= cleanQuery.length; j++) matrix[0][j] = j;

    for (let i = 1; i <= cleanText.length; i++) {
        for (let j = 1; j <= cleanQuery.length; j++) {
            matrix[i][j] = cleanText[i - 1] === cleanQuery[j - 1]
                ? matrix[i - 1][j - 1]
                : Math.min(
                    matrix[i - 1][j - 1] + 1,
                    matrix[i][j - 1] + 1,
                    matrix[i - 1][j] + 1,
                );
        }
    }

    const distance = matrix[cleanText.length][cleanQuery.length];
    return Math.max(
        0,
        1 - distance / Math.max(cleanQuery.length, cleanText.length),
    );
}

function showSuggestions(inputId, listId, key) {
    const input = document.getElementById(inputId);
    const list = document.getElementById(listId);
    let currentMatches = [];
    let selectedIndex = 0;

    const selectMatch = (match) => {
        input.value = match.text;
        list.style.display = "none";
        selectedIndex = 0;

        // Auto-populate author if title was selected
        if (key === "title") {
            document.getElementById("author").value = match.book.author;
        }
    };

    const updateSelection = () => {
        const items = list.querySelectorAll("li");
        items.forEach((item, index) => {
            if (index === selectedIndex) {
                item.classList.add("selected");
            } else {
                item.classList.remove("selected");
            }
        });
    };

    const updateSuggestions = () => {
        const query = input.value.trim().toLowerCase();
        list.innerHTML = "";
        selectedIndex = 0;
        if (!query) {
            list.style.display = "none";
            currentMatches = [];
            return;
        }

        // Fuzzy match
        const matches = books
            .map((book) => ({
                text: book[key],
                book: book,
                score: fuzzyMatch(query, book[key]),
            }))
            .filter((item) => item.score > 0.3)
            .sort((a, b) => b.score - a.score)
            .slice(0, 8);

        currentMatches = matches;

        if (matches.length === 0) {
            list.style.display = "none";
            return;
        }

        matches.forEach((match, index) => {
            const li = document.createElement("li");
            li.textContent = match.text;
            if (index === 0) li.classList.add("selected");
            li.addEventListener("click", () => selectMatch(match));
            list.appendChild(li);
        });

        list.style.display = "block";
    };

    input.addEventListener("input", updateSuggestions);

    // Handle keyboard navigation
    input.addEventListener("keydown", (e) => {
        if (currentMatches.length === 0) return;

        if (e.key === "ArrowDown") {
            e.preventDefault();
            selectedIndex = (selectedIndex + 1) %
                currentMatches.length;
            updateSelection();
        } else if (e.key === "ArrowUp") {
            e.preventDefault();
            selectedIndex = (selectedIndex - 1 + currentMatches.length) %
                currentMatches.length;
            updateSelection();
        } else if (e.key === "Enter" || e.key === "Tab") {
            if (list.style.display === "block") {
                e.preventDefault();
                selectMatch(currentMatches[selectedIndex]);
            }
        } else if (e.key === "Escape") {
            list.style.display = "none";
            selectedIndex = 0;
        }
    });

    // Hide when clicking away
    document.addEventListener("click", (e) => {
        if (!list.contains(e.target) && e.target !== input) {
            list.style.display = "none";
            selectedIndex = 0;
        }
    });

    // Return function to trigger suggestions externally
    return { updateSuggestions };
}

// Store references to update functions
const titleSuggestions = showSuggestions("title", "titleSuggestions", "title");
const authorSuggestions = showSuggestions(
    "author",
    "authorSuggestions",
    "author",
);

// Handle author selection
document.getElementById("author").addEventListener("change", () => {
    const selectedAuthor = document.getElementById("author").value.trim();
    if (!selectedAuthor) return;

    // Find all books by this author
    const booksByAuthor = books.filter((book) =>
        book.author.toLowerCase() === selectedAuthor.toLowerCase()
    );

    const titleInput = document.getElementById("title");

    if (booksByAuthor.length === 1) {
        // Auto-populate if only one book
        titleInput.value = booksByAuthor[0].title;
    } else if (booksByAuthor.length > 1) {
        // Show suggestions for this author's books
        titleInput.focus();
        titleInput.dispatchEvent(new Event("input"));
    }
});

// Load data on page load
loadData();
