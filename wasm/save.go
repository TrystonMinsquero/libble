package main

import (
	"errors"
	"reflect"

	. "libble/shared"
)

func saveKey(jsonName string) string {
	return "libble." + jsonName
}

type FieldPredicate func(string) bool

const libbleIDKey = "libble.id"

func saveLibbleID(libbleID string) {
	err := saveData(saveKey(libbleIDKey), libbleID)
	log(err, "Failed to save libble id "+libbleID)
}

func loadLibbleID() string {
	id, err := loadData(saveKey(libbleIDKey))
	log(err, "Failed to load libble id "+id)
	return id
}

func isStaticSaveDataField(jsonFieldName string) bool {
	switch jsonFieldName {
	case "books":
		return true
	case "quotes":
		return true
	}
	return false
}

func saveAllDataFiltered(data SaveData, filter FieldPredicate) error {
	v := reflect.ValueOf(data)
	t := v.Type()
	var err error
	err = nil
	for i := range t.NumField() {
		field := t.Field(i)
		if !field.IsExported() {
			continue
		}
		jsonName := field.Tag.Get("json")
		if jsonName != "" && filter(jsonName) {
			err = errors.Join(err, saveJson(saveKey(jsonName), v.Field(i).Interface()))
		}
	}
	return err
}

func saveAllData(data SaveData) error {
	return saveAllDataFiltered(data, func(s string) bool { return true })
}

func saveNonStaticData(data SaveData) error {
	return saveAllDataFiltered(data, func(s string) bool { return !isStaticSaveDataField(s) })
}

func canPlay() bool {
	return loadLibbleID() != ""
}

func loadAllDataFiltered(data *SaveData, filter FieldPredicate) error {
	pv := reflect.ValueOf(data)
	v := pv.Elem()
	t := v.Type()
	var err error
	err = nil
	for i := range t.NumField() {
		fieldType := t.Field(i)
		if !fieldType.IsExported() {
			continue
		}
		jsonName := fieldType.Tag.Get("json")
		field := v.Field(i).Addr().Interface()
		if jsonName != "" && filter(jsonName) {
			err = errors.Join(err, loadJson(saveKey(jsonName), field))
		}
	}
	data.PopulateLookups()
	return err
}

func loadNonStaticData(data *SaveData) error {
	return loadAllDataFiltered(data, func(s string) bool { return isStaticSaveDataField(s) })
}

func loadAllData(data *SaveData) error {
	return loadAllDataFiltered(data, func(s string) bool { return true })
}
