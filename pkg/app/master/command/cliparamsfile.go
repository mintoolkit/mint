package command

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"reflect"
	"strconv"
	"strings"

	"github.com/mintoolkit/mint/pkg/app"
	"github.com/urfave/cli/v2"
)

func ParseParamsFromFile(ctx *cli.Context, xc *app.ExecutionContext, filePath string, validFlags []cli.Flag) {
	params, err := parseJsonFileParams(filePath)

	if err != nil {
		xc.Out.Error("params.file.error", err.Error())
		xc.Out.State("exited",
			ovars{
				"exit.code": -1,
			})
		xc.Exit(-1)
	}

	err = setFileParams(ctx, xc, params, validFlags)

	if err != nil {
		xc.Out.Error("params.file.param.value.error", err.Error())
		xc.Out.State("exited",
			ovars{
				"exit.code": -1,
			})
		xc.Exit(-1)
	}

}

func validFlagsMap(xc *app.ExecutionContext, validFlags []cli.Flag) map[string]cli.Flag {
	validFlagsMap := make(map[string]cli.Flag)

	for _, flag := range validFlags {
		if len(flag.Names()) == 0 {
			xc.Out.Error("params-file.validate.params", "flag names has no values")
			xc.Out.State("exited",
				ovars{
					"exit.code": -1,
				})
			xc.Exit(-1)
		}

		validFlagsMap[flag.Names()[0]] = flag
	}

	return validFlagsMap
}

func flagSetString(flag cli.Flag, paramValue interface{}) (string, error) {
	paramValueType := reflect.TypeOf(paramValue).Kind()

	switch flag.(type) {
	case *cli.StringFlag:
		if paramValueType == reflect.String {
			return paramValue.(string), nil
		} else {
			return "", fmt.Errorf("expected string value found: %v for flag %s", paramValueType, flag.Names()[0])
		}
	case *cli.BoolFlag:
		if paramValueType == reflect.Bool {
			return strconv.FormatBool(paramValue.(bool)), nil
		} else {
			return "", fmt.Errorf("expected boolean value found: %v for flag %s", paramValueType, flag.Names()[0])
		}
	case *cli.IntFlag:
		// JSON Unmarshal unmarshals a JSON number as a Float64 by default
		if paramValueType == reflect.Int {
			return strconv.Itoa(paramValue.(int)), nil
		} else if paramValueType == reflect.Float64 {
			// Decimal will be truncated out here
			return strconv.Itoa(int(paramValue.(float64))), nil
		} else {
			return "", fmt.Errorf("expected Int value found: %v for flag %s", paramValueType, flag.Names()[0])
		}
	case *cli.StringSliceFlag:
		// Parse both string slice or string values
		if paramValueType == reflect.String {
			return paramValue.(string), nil
		} else if paramValueType == reflect.Slice {
			var values []string
			slice := reflect.ValueOf(paramValue)
			for i := 0; i < slice.Len(); i++ {
				values = append(values, fmt.Sprintf("%v", slice.Index(i).Interface()))
			}
			return strings.Join(values, ","), nil
		} else {
			return "", fmt.Errorf("expected string or string slice value found: %v for flag %s", paramValueType, flag.Names()[0])
		}
	default:
		return "", errors.New("flag type not supported by params file")
	}
}

func setFileParams(ctx *cli.Context, xc *app.ExecutionContext, params map[string]interface{}, validFlags []cli.Flag) error {
	validFlagsMap := validFlagsMap(xc, validFlags)

	for fileParamKey, fileParamValue := range params {

		if flag, ok := validFlagsMap[fileParamKey]; !ok {
			return fmt.Errorf("the command params file contains an invalid flag - %s", fileParamKey)
		} else {
			setValue, err := flagSetString(flag, fileParamValue)
			if err != nil {
				return err
			}

			if ctx.IsSet(fileParamKey) {
				xc.Out.Info("command.params.file",
					ovars{
						"message": "updating already set value from params file",
						"param":   fileParamKey,
					})
			}

			// The Parameter key and value has now been sanitized and the values can be set
			ctx.Set(fileParamKey, setValue)
		}
	}

	return nil
}

func parseJsonFileParams(filepath string) (map[string]interface{}, error) {
	fileContent, err := os.ReadFile(filepath)

	if err != nil {
		return nil, err
	}

	var params map[string]interface{}
	err = json.Unmarshal(fileContent, &params)

	if err != nil {
		return nil, err
	}

	return params, nil
}
