# Data Updater
Go sources for binary, capable of updating schedule, substitution and menu data for Gimnazija Vič and writing it to MySql database, compatible with GimVic server app.

Usage: `./whateverYourBinaryNameIs [arg]` where arg could be sch (to update schedule), sub (to update substitutions) or menu (to update menu). When updating menu, csv source file must be provided as second argument. If all 3 args are provided, only the first one will be used.