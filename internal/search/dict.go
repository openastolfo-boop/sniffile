package search

import (
	"github.com/kopeyka/sniffile/internal/index"
)

// A tiny bilingual thesaurus. It is not a language model: it exists so that
// "фотка с мопсом" has a chance against files named pug_01.jpg, and so that
// "cute dog" also wakes up puppy photos. Extend it by adding one line.
var rawDict = map[string]string{
	// photography / screens
	"photo":       "image picture photograph shot",
	"image":       "picture photo",
	"picture":     "image photo",
	"фото":        "photo image picture",
	"фотка":       "photo image picture",
	"фотография":  "photo image picture",
	"картинка":    "picture image photo",
	"изображение": "image picture",
	"скриншот":    "screenshot screen capture",
	"скрин":       "screenshot screen capture",
	"screenshot":  "screen capture",
	"обои":        "wallpaper background",
	"wallpaper":   "background",
	"аватар":      "avatar profile picture",
	"avatar":      "profile picture",
	"логотип":     "logo brand",
	"logo":        "brand",
	"иконка":      "icon",
	"icon":        "icon",

	// video / audio
	"video":   "clip footage movie",
	"видео":   "video clip footage",
	"ролик":   "video clip",
	"клип":    "clip video",
	"фильм":   "movie film video",
	"movie":   "film video",
	"музыка":  "music audio song",
	"music":   "audio song",
	"песня":   "song music audio",
	"аудио":   "audio music sound",
	"звук":    "sound audio",
	"podcast": "podcast audio",

	// animals
	"собака":   "dog puppy",
	"собаки":   "dog puppy",
	"пёс":      "dog",
	"пес":      "dog",
	"щенок":    "puppy dog",
	"мопс":     "pug dog",
	"корги":    "corgi dog",
	"кот":      "cat",
	"кошка":    "cat kitten",
	"котенок":  "kitten cat",
	"котёнок":  "kitten cat",
	"dog":      "puppy canine",
	"puppy":    "dog",
	"cat":      "kitten",
	"kitten":   "cat",
	"животное": "animal wildlife",
	"animal":   "wildlife",
	"птица":    "bird",
	"bird":     "animal",
	"лиса":     "fox",
	"fox":      "animal",
	"слон":     "elephant",
	"elephant": "animal",
	"медведь":  "bear",
	"bear":     "animal",

	// people
	"человек":  "person people human",
	"люди":     "people person",
	"мужчина":  "man person",
	"женщина":  "woman person",
	"девушка":  "woman girl person",
	"парень":   "man guy person",
	"ребенок":  "child kid person",
	"ребёнок":  "child kid person",
	"портрет":  "portrait face person",
	"portrait": "face person",
	"селфи":    "selfie person face",
	"selfie":   "person face",
	"группа":   "group people team",
	"group":    "people team",
	"person":   "people human",
	"people":   "person group",

	// places
	"город":       "city urban street",
	"city":        "urban street",
	"улица":       "street city",
	"street":      "city road",
	"дом":         "house home building",
	"house":       "home building",
	"здание":      "building architecture house",
	"building":    "architecture house",
	"квартира":    "apartment room home",
	"комната":     "room interior",
	"room":        "interior",
	"офис":        "office workplace desk",
	"office":      "workplace desk",
	"школа":       "school",
	"университет": "university campus",
	"машина":      "car automobile vehicle",
	"авто":        "car automobile vehicle",
	"автомобиль":  "car automobile",
	"car":         "automobile vehicle",
	"мотоцикл":    "motorcycle bike",
	"велосипед":   "bicycle bike",
	"bike":        "bicycle motorcycle",

	// nature
	"природа":  "nature landscape",
	"nature":   "landscape",
	"лес":      "forest woods trees",
	"forest":   "woods trees",
	"дерево":   "tree wood",
	"tree":     "wood",
	"гора":     "mountain",
	"горы":     "mountains landscape",
	"mountain": "landscape",
	"море":     "sea ocean beach water",
	"sea":      "ocean beach water",
	"океан":    "ocean sea",
	"река":     "river water",
	"river":    "water",
	"озеро":    "lake water",
	"пляж":     "beach sea sand",
	"beach":    "sea sand ocean",
	"небо":     "sky clouds",
	"sky":      "clouds",
	"облака":   "clouds sky",
	"закат":    "sunset dusk sky",
	"sunset":   "dusk sky",
	"рассвет":  "sunrise dawn sky",
	"ночь":     "night dark",
	"night":    "dark",
	"снег":     "snow winter",
	"snow":     "winter",
	"дождь":    "rain wet",
	"rain":     "wet",
	"зима":     "winter snow",
	"лето":     "summer",
	"осень":    "autumn fall",
	"весна":    "spring",
	"цветы":    "flowers plants",
	"цветок":   "flower plant",
	"flowers":  "flower plant",
	"трава":    "grass green",

	// food / objects
	"еда":      "food meal",
	"food":     "meal",
	"пицца":    "pizza food",
	"pizza":    "food",
	"кофе":     "coffee cup drink",
	"coffee":   "cup drink",
	"чай":      "tea drink",
	"торт":     "cake dessert food",
	"cake":     "dessert food",
	"хлеб":     "bread food",
	"бургер":   "burger food",
	"напиток":  "drink beverage",
	"бутылка":  "bottle drink",
	"книга":    "book read",
	"book":     "read",
	"текст":    "text document",
	"документ": "document doc",
	"document": "doc",
	"код":      "code source programming",
	"code":     "source programming",
	"проект":   "project code",
	"игра":     "game gaming",
	"игры":     "games game",
	"game":     "gaming",
	"мем":      "meme funny",
	"meme":     "funny",

	// adjectives
	"красивый":  "beautiful pretty nice",
	"красивая":  "beautiful pretty",
	"beautiful": "pretty nice",
	"милый":     "cute adorable",
	"милая":     "cute adorable",
	"cute":      "adorable pretty",
	"смешной":   "funny",
	"funny":     "humor",
	"темный":    "dark",
	"тёмный":    "dark",
	"dark":      "night",
	"светлый":   "light bright",
	"большой":   "big large",
	"big":       "large",
	"маленький": "small tiny",
	"small":     "tiny",
	"старый":    "old retro vintage",
	"old":       "retro vintage",
	"новый":     "new",
	"new":       "fresh",
}

// dict maps a stemmed word to stemmed synonyms.
var dict = func() map[string][]string {
	out := make(map[string][]string, len(rawDict)*2)
	for k, v := range rawDict {
		key := index.Stem(index.Fold(k))
		out[key] = appendUnique(out[key], index.Tokenize(v)...)
	}
	return out
}()

// Synonyms returns extra terms for a stemmed query word.
func Synonyms(stem string) []string {
	return dict[stem]
}

func appendUnique(dst []string, words ...string) []string {
	for _, w := range words {
		dup := false
		for _, d := range dst {
			if d == w {
				dup = true
				break
			}
		}
		if !dup {
			dst = append(dst, w)
		}
	}
	return dst
}
