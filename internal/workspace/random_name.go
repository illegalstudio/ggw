package workspace

import (
	"fmt"
	"math/rand/v2"
	"os"

	"github.com/illegalstudio/ggw/internal/layout"
	"github.com/illegalstudio/ggw/internal/worktree"
)

// randIntN is the RNG hook used by RandomName; tests may override it.
var randIntN = rand.IntN

// adjectives and nouns form Docker-style names: "intelligent-elephant".
var adjectives = []string{
	"admiring", "adoring", "affectionate", "agitated", "amazing",
	"angry", "awesome", "beautiful", "blissful", "bold",
	"boring", "brave", "busy", "charming", "clever",
	"cool", "compassionate", "competent", "confident", "cranky",
	"crazy", "dazzling", "determined", "distracted", "dreamy",
	"eager", "ecstatic", "elastic", "elated", "elegant",
	"eloquent", "epic", "exciting", "fervent", "festive",
	"flamboyant", "focused", "friendly", "frosty", "funny",
	"gallant", "gifted", "goofy", "gracious", "great",
	"happy", "hardcore", "heuristic", "hopeful", "hungry",
	"infallible", "inspiring", "intelligent", "interesting", "jolly",
	"jovial", "keen", "kind", "laughing", "loving",
	"lucid", "magical", "modest", "musing", "mystifying",
	"naughty", "nervous", "nice", "nifty", "nostalgic",
	"objective", "optimistic", "peaceful", "pedantic", "pensive",
	"practical", "priceless", "quirky", "quizzical", "recursing",
	"relaxed", "reverent", "romantic", "sad", "serene",
	"sharp", "silly", "sleepy", "stoic", "strange",
	"stupefied", "suspicious", "sweet", "tender", "thirsty",
	"trusting", "unruffled", "upbeat", "vibrant", "vigilant",
	"vigorous", "wizardly", "wonderful", "xenodochial", "youthful",
	"zealous", "zen",
}

var nouns = []string{
	"alligator", "ant", "anteater", "antelope", "armadillo",
	"badger", "bat", "bear", "beaver", "bee",
	"bison", "boar", "buffalo", "butterfly", "camel",
	"capybara", "cat", "caterpillar", "cheetah", "chicken",
	"chimpanzee", "chinchilla", "cobra", "coyote", "crab",
	"crane", "crocodile", "crow", "deer", "dingo",
	"dog", "dolphin", "donkey", "dove", "dragon",
	"duck", "eagle", "echidna", "eel", "elephant",
	"elk", "emu", "falcon", "ferret", "finch",
	"flamingo", "fox", "frog", "gazelle", "gecko",
	"giraffe", "goat", "goose", "gorilla", "grasshopper",
	"hamster", "hare", "hawk", "hedgehog", "heron",
	"hippo", "hornet", "horse", "hyena", "ibis",
	"iguana", "impala", "jackal", "jaguar", "jellyfish",
	"kangaroo", "koala", "komodo", "lemur", "leopard",
	"lion", "lizard", "llama", "lobster", "lynx",
	"macaw", "magpie", "manatee", "meerkat", "mole",
	"mongoose", "monkey", "moose", "mouse", "narwhal",
	"newt", "octopus", "okapi", "opossum", "ostrich",
	"otter", "owl", "ox", "oyster", "panda",
	"panther", "parrot", "peacock", "pelican", "penguin",
	"phoenix", "pigeon", "platypus", "pony", "porcupine",
	"quail", "rabbit", "raccoon", "raven", "rhino",
	"salamander", "salmon", "seal", "shark", "sheep",
	"shrimp", "skunk", "sloth", "snail", "snake",
	"sparrow", "spider", "squid", "squirrel", "starfish",
	"stingray", "stork", "swan", "tiger", "toad",
	"tortoise", "toucan", "trout", "turkey", "turtle",
	"viper", "vulture", "walrus", "wasp", "weasel",
	"whale", "wolf", "wombat", "woodpecker", "worm",
	"yak", "zebra",
}

// RandomName returns a Docker-style name: adjective-noun, e.g. "intelligent-elephant".
func RandomName() string {
	return adjectives[randIntN(len(adjectives))] + "-" + nouns[randIntN(len(nouns))]
}

// UniqueRandomName returns a RandomName that is free as a local branch,
// origin tracking branch, and workspace destination path for (org, repo).
// It retries a bounded number of times to avoid rare collisions.
func UniqueRandomName(repoPath, org, repo string) (string, error) {
	const maxAttempts = 64
	for range maxAttempts {
		name := RandomName()
		if worktree.BranchExistsLocal(repoPath, name) {
			continue
		}
		if worktree.RemoteBranchRef(repoPath, name) != "" {
			continue
		}
		dest, err := layout.WorkspacePath(org, repo, layout.SlugifyBranch(name))
		if err != nil {
			return "", err
		}
		if _, err := os.Stat(dest); err == nil {
			continue
		} else if !os.IsNotExist(err) {
			return "", fmt.Errorf("check workspace path %s: %w", dest, err)
		}
		return name, nil
	}
	return "", fmt.Errorf("could not generate a unique random branch name after %d attempts", maxAttempts)
}
