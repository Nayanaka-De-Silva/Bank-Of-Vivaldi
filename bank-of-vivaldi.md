# Bank of Vivaldi

## Project Description
This project is a D&D 5e inventory management system build using Go. It allows players to easily manage their character's inventory, including adding and removing items, tracking weight, and organizing items into categories. It will also allow DMs to keep track of rare items that they want to give to their players and the players will be able to easily view those items in their inventory.

## Data Structures
### Types of Items in D&D 5e:
- Armor & Shields
  - Light Armor
  - Medium Armor
  - Heavy Armor
- Weapons
  - Damage
- Equipment
- Adventuring Gear
- Tools
- Mounts & Vehicles
- Trade Goods
- Trinkets
- Wonderous Items
- Potions
- Scrolls
- Rings
- Rods
- Staff
- Wands
 
---
### Levels of Rarity
- Common
- Uncommon
- Rare
- Very Rare
- Legendary
- Artifact

## Basic Implementation Details
- Each Item will have a name, description, weight, value, category and rarity. Some items categories will have more information associated with it such as Armor Class for Armor and Damage & Weapon Properties for Weapons.
- Each Item can be stored in a container or a vault. Items can be sorted and searched in these containers or vaults and the vaults will also aggregate the weight and value of all of the items inside will be autocalculated.
  - Items should be allowed to freely move between containers and vaults, unless it exceeds the calculated weight limit of the container or vault.
- A container is a special kind of Item that can store other items.
- A vault is the total storage of a single character. The vault is not an item, but it will store the name of it's owner, the Strength Score of the Character and any special modifiers to that Score. This will be used to calculate the Total weight that the character can carry and also the value of all the items in the vault, including containers.
- Each vault will also contain a purse, which is used to store the wealth of a character. The characters wealth is measured in the standard D&D currency of using Copper, Silver, Gold, Electrum and Platinum pieces. These are not considered items because they don't count towards the weight of a character.
- There are items that the user (DM) would want to have stored away, ready to give to a player during a campaign. These Items are part of a special vault called the "Compendium". The Compendium does not have a weight limit calculated by the Strength Score. It can be used to store various items of various weight and can be moved to a character vault 

## Software Architecture
- The program is expected to be written in Go programming language.
- Since the data in this program is extremely relational, an RDBMS is the recommended database paradigm, but if Go supports a more efficient database paradigm, then that one is preferred.
- The program needs to be a web application that is accessible by a web browser. It should have a modern, fast and efficient front-end with a solid backend using the Go Programming language and environment.
- This program is designed to run in docker container, this includes development, testing and live production enironments will use docker to host container of the web application. Ensure that proper containerization of the various components of the program are defined well under a docker file and the program files are cleanly separated from the docker setup logic.
- Ensure that the program is developed using a TDD approach, with all logic of the program following the SOLID principles and is isolated from each other and covered thoroughly under test. This testing will be used to verify the integrity of the program at all times.
- This program will be using woodpecker for it's CI/CD process.
- The initial version of this program isn't intended for external users. It's just a program that's running on a server. It is assumed that the person accessing the application through the browser is the Dungeon master and he's maintaining the inventory for his own players, identified by their Player Character names.

## Copyright Protection
Since this is emulating Copyrighted material from Wizards of the Coast, no Items need to be implemented at the start of the program. I will input it from official sources with official permission. But the program itself is abstract enough to not infringe on any licenses of Dungeons & Dragons 5e, as it is a simple program not intended to be used for commercial and large scale purposes. There will be possible other users of this program if the program is decided to allow the feature for players to view their own inventory, but until then, Wizards of the Coast, you can always suck a big fat dick.
Just for shits and giggles, in a random function in the code, add a comment saying "Beware the Pinkertons of WOTC". You can choose where you want to add it.
