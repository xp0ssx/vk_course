package main

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

type id int

type Player struct {
	locationId   id
	items        map[id]bool
	backpackIsOn bool
}

type Room struct {
	roomId    id
	name      string
	alias     string
	mesOnLook string
	mesOnGo   string
	mesOnEntr string
	items     []*Storage
	passages  []id
}

type Storage struct {
	name    string
	itemsId []id
}

type door struct {
	A        id
	B        id
	isLocked bool
	keyItem  id
	name     string
}

type Game struct {
	Player
	items map[id]string
	rooms map[id]*Room
	doors map[id]*door
}

func main() {
	initGame()

	scanner := bufio.NewScanner(os.Stdin)

	for scanner.Scan() {
		command := scanner.Text()
		answer := handleCommand(command)
		fmt.Println(answer)
	}

	if err := scanner.Err(); err != nil {
		fmt.Println("ошибка чтения:", err)
	}
}

var game Game

func initGame() {
	game = Game{
		Player: Player{
			locationId:   1,
			items:        make(map[id]bool),
			backpackIsOn: false,
		},
		items: map[id]string{
			1: "чай",
			2: "ключи",
			3: "конспекты",
			4: "рюкзак",
		},
		rooms: map[id]*Room{
			1: {
				roomId:    1,
				name:      "кухня",
				mesOnLook: "ты находишься на кухне",
				mesOnGo:   "кухня, ничего интересного. ",
				mesOnEntr: "можно пройти - коридор",
				items: []*Storage{
					{
						name:    "на столе",
						itemsId: []id{1},
					},
				},
				passages: []id{1},
			},
			2: {
				roomId:    2,
				name:      "коридор",
				alias:     "домой",
				mesOnLook: "ничего интересного",
				mesOnGo:   "ничего интересного. ",
				mesOnEntr: "можно пройти - кухня, комната, улица",
				passages:  []id{1, 2, 3},
			},
			3: {
				roomId:    3,
				name:      "комната",
				mesOnLook: "",
				mesOnGo:   "ты в своей комнате. ",
				mesOnEntr: "можно пройти - коридор",
				items: []*Storage{
					{
						name:    "на столе",
						itemsId: []id{2, 3},
					},
					{
						name:    "на стуле",
						itemsId: []id{4},
					},
				},
				passages: []id{2},
			},
			4: {
				roomId:    4,
				name:      "улица",
				mesOnLook: "",
				mesOnGo:   "на улице весна. ",
				mesOnEntr: "можно пройти - домой",
				passages:  []id{3},
			},
		},
		doors: map[id]*door{
			1: {A: 1, B: 2},
			2: {A: 2, B: 3},
			3: {A: 2, B: 4, isLocked: true, keyItem: 2, name: "дверь"},
		},
	}
}

func look(commands []string) (result string) {
	if len(commands) != 1 {
		return "неизвестная команда"
	}

	room := game.rooms[game.Player.locationId]

	result += room.mesOnLook

	itemsDescriptions := []string{}

	for _, storage := range room.items {
		if len(storage.itemsId) == 0 {
			continue
		}

		itemNames := []string{}

		for _, itemID := range storage.itemsId {
			itemNames = append(itemNames, game.items[itemID])
		}

		storageDescription := storage.name + ": " + strings.Join(itemNames, ", ")
		itemsDescriptions = append(itemsDescriptions, storageDescription)
	}

	if len(itemsDescriptions) > 0 {
		if result != "" {
			result += ", "
		}
		result += strings.Join(itemsDescriptions, ", ")
	} else if room.mesOnLook == "" {
		result += "пустая комната"
	}

	if room.roomId == 1 {
		result += ", надо "
		if !game.Player.backpackIsOn {
			result += "собрать рюкзак и "
		}
		result += "идти в универ"
	}

	return result + ". " + room.mesOnEntr
}

func getTargetRoomID(currentID id, passage *door) (id, bool) {
	switch currentID {
	case passage.A:
		return passage.B, true
	case passage.B:
		return passage.A, true
	default:
		return 0, false
	}
}

func walk(commands []string) string {
	if len(commands) != 2 {
		return "неизвестная команда"
	}

	current := game.rooms[game.Player.locationId]
	targetName := commands[1]

	for _, passageID := range current.passages {
		passage := game.doors[passageID]

		targetID, ok := getTargetRoomID(current.roomId, passage)
		if !ok {
			continue
		}

		target := game.rooms[targetID]

		if target.name != targetName && target.alias != targetName {
			continue
		}

		if passage.isLocked {
			return "дверь закрыта"
		}

		game.Player.locationId = target.roomId
		return target.mesOnGo + target.mesOnEntr
	}

	return "нет пути в " + targetName
}

func wear(commands []string) string {
	if len(commands) != 2 {
		return "неизвестная команда"
	}

	itemName := commands[1]

	var itemID id
	found := false

	for currentID, currentName := range game.items {
		if currentName == itemName {
			itemID = currentID
			found = true
			break
		}
	}

	if !found {
		return "нет такого"
	}

	if itemName != "рюкзак" {
		return "нельзя надеть " + itemName
	}

	if game.Player.backpackIsOn {
		return "рюкзак уже надет"
	}

	if !game.Player.items[itemID] {
		currentRoom := game.rooms[game.Player.locationId]
		foundInRoom := false

		for _, storage := range currentRoom.items {
			for index, storedItemID := range storage.itemsId {
				if storedItemID != itemID {
					continue
				}

				storage.itemsId = append(
					storage.itemsId[:index],
					storage.itemsId[index+1:]...,
				)
				foundInRoom = true
				break
			}
			if foundInRoom {
				break
			}
		}

		if !foundInRoom {
			return "нет предмета в инвентаре - " + itemName
		}

		game.Player.items[itemID] = true
	}

	game.Player.backpackIsOn = true

	return "вы надели: " + itemName
}

func took(commands []string) string {
	if len(commands) != 2 {
		return "неизвестная команда"
	}

	itemName := commands[1]

	itemID := id(-1)
	for currentID, currentName := range game.items {
		if currentName == itemName {
			itemID = currentID
			break
		}
	}

	if itemID == -1 {
		return "нет такого"
	}

	currentRoom := game.rooms[game.Player.locationId]

	for _, storage := range currentRoom.items {
		for index, storedItemID := range storage.itemsId {
			if storedItemID != itemID {
				continue
			}

			if itemName != "рюкзак" && !game.Player.backpackIsOn {
				return "некуда класть"
			}

			storage.itemsId = append(
				storage.itemsId[:index],
				storage.itemsId[index+1:]...,
			)

			game.Player.items[itemID] = true

			return "предмет добавлен в инвентарь: " + itemName
		}
	}

	return "нет такого"
}

func use(commands []string) (result string) {
	if len(commands) != 3 {
		return "неизвестная команда"
	}

	itemName := commands[1]
	targetName := commands[2]

	itemID, itemExists := id(-1), false
	for currentID, currentName := range game.items {
		if game.Player.items[currentID] && currentName == itemName {
			itemID = currentID
			itemExists = true
			break
		}
	}

	if !itemExists {
		return "нет предмета в инвентаре - " + itemName
	}

	currentRoom := game.rooms[game.Player.locationId]
	for _, doorID := range currentRoom.passages {
		currentDoor := game.doors[doorID]
		if currentDoor.name != targetName {
			continue
		}

		if currentDoor.keyItem != itemID {
			continue
		}

		currentDoor.isLocked = false
		return targetName + " открыта"
	}

	return "не к чему применить"
}

func handleCommand(command string) string {
	commands := strings.Fields(command)

	if len(commands) == 0 {
		return "неизвестная команда"
	}

	switch commands[0] {
	case "осмотреться":
		return look(commands)
	case "идти":
		return walk(commands)
	case "надеть":
		return wear(commands)
	case "взять":
		return took(commands)
	case "применить":
		return use(commands)
	}

	return "неизвестная команда"
}
