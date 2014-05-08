package common

import (
    "reflect"
    "strings"
    "fmt"
)

func Map(models interface{}, fieldName string) []interface{} {
    fields := []interface{}{}
    modelsVal := reflect.ValueOf(models)
    for i := 0; i < modelsVal.Len(); i++ {
        modelVal := modelsVal.Index(i)
        fieldVal := modelVal.Elem().FieldByName(fieldName)
        if !fieldVal.IsValid() {
            panic(NewError("Model struct %v has no field %v", modelVal.Elem().Type().Name(), fieldName))
        }
        field := fieldVal.Interface()
        fields = append(fields, field)
    }
    return fields
}

func MapStr(models interface{}, fieldName string) []string {
    fields := Map(models, fieldName)
    fieldsStr := []string{}
    for _, field := range fields {
        fieldsStr = append(fieldsStr, field.(string))
    }
    return fieldsStr
}

func MapInt64(models interface{}, fieldName string) []int64 {
    fields := Map(models, fieldName)
    fieldsStr := []int64{}
    for _, field := range fields {
        fieldsStr = append(fieldsStr, field.(int64))
    }
    return fieldsStr
}

func ToIArray(items interface{}) []interface{} {
    iArray := []interface{}{}
    itemsVal := reflect.ValueOf(items)
    for i := 0; i < itemsVal.Len(); i++ {
        itemVal := itemsVal.Index(i)
        iArray = append(iArray, itemVal.Interface())
    }
    return iArray
}

func ToSArray(items interface{}) []string {
    sArray := []string{}
    itemsVal := reflect.ValueOf(items)
    for i := 0; i < itemsVal.Len(); i++ {
        itemVal := itemsVal.Index(i)
        sArray = append(sArray, fmt.Sprintf("%v", itemVal.Interface()))
    }
    return sArray
}

func Placeholders(num int) string {
    return "?" + strings.Repeat(",?", num-1)
}

func Flatten(items ...interface{}) []interface{} {
    iArray := []interface{}{}
    for _, item := range items {
        itemKind := reflect.TypeOf(item).Kind()
        if itemKind == reflect.Array || itemKind == reflect.Slice {
            itemArray := []interface{}{}
            itemsVal := reflect.ValueOf(items)
            for i := 0; i < itemsVal.Len(); i++ {
                itemVal := itemsVal.Index(i)
                itemArray = append(itemArray, itemVal.Interface())
            }
            iArray = append(iArray, Flatten(itemArray)...)
        } else {
            iArray = append(iArray, item)
        }
    }
    return iArray
}
