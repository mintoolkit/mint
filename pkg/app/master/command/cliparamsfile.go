package command

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
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

func validFlagsMap(validFlags []cli.Flag) (map[string]cli.Flag, error) {
	validFlagsMap := make(map[string]cli.Flag)

	for _, flag := range validFlags {
		if len(flag.Names()) == 0 {
			return nil, errors.New("flag names has no values")
		}

		validFlagsMap[flag.Names()[0]] = flag
	}

	return validFlagsMap, nil
}

func flagSetString(flag cli.Flag, paramValue interface{}) (string, error) {
	switch flag.(type) {
	case *cli.StringFlag:
		switch paramValueType := paramValue.(type) {
		case string:
			return paramValueType, nil
		default:
			return "", fmt.Errorf("expected string value found: %T for flag %s", paramValueType, flag.Names()[0])
		}
	case *cli.BoolFlag:
		switch paramValueType := paramValue.(type) {
		case bool:
			return strconv.FormatBool(paramValueType), nil
		default:
			return "", fmt.Errorf("expected boolean value found: %T for flag %s", paramValueType, flag.Names()[0])
		}
	case *cli.IntFlag:
		switch paramValueType := paramValue.(type) {
		case int:
			return strconv.Itoa(paramValueType), nil
		case float64:
			//JSON unmarshal unmarshals a JSON number as a float64 by default
			return strconv.Itoa(int(paramValueType)), nil
		default:
			return "", fmt.Errorf("expected Int value found: %T for flag %s", paramValueType, flag.Names()[0])
		}
	case *cli.StringSliceFlag:
		switch paramValueType := paramValue.(type) {
		case string:
			return paramValueType, nil
		case []string:
			return strings.Join(paramValueType, ","), nil
		default:
			return "", fmt.Errorf("expected string or string array value found: %T for flag %s", paramValueType, flag.Names()[0])
		}
	default:
		return "", errors.New("flag type not supported by params file")
	}
}

func setFileParams(ctx *cli.Context, xc *app.ExecutionContext, params map[string]interface{}, validFlags []cli.Flag) error {
	validFlagsMap, err := validFlagsMap(validFlags)

	if err != nil {
		return err
	}

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
