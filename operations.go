package main

import (
	"fmt"
	"strconv"

	log "github.com/sirupsen/logrus"
	"github.com/vektah/gqlparser/v2/ast"
	"golang.org/x/exp/maps"
)

const (
	MAX_PATH_DEPTH = 3
)

type TypeSignature struct {
	Type     string
	Default  interface{}
	Required bool
}

type FieldPathDetail struct {
	Path     string
	IDInPath bool
}

type GetMethod struct {
	Path             string                   // path for REST router
	Method           string                   // GET/POST
	IDInPath         bool                     // Whether the ID is encoded into path
	QueryString      map[string]TypeSignature // for validating QS
	GQLQuery         string                   // name of the underlying GQL query
	ResultSelections []string                 // What is the full selection set of the GQL response
	OriginalField    string
	FieldPath        []FieldPathDetail // parent type path for this field
}

type PostMethod struct{}

// MakeTypeSig - create type sig object with stored default values
func MakeTypeSig(name, typeName string, required bool, defaultValue *ast.Value) TypeSignature {

	ts := TypeSignature{
		Type:     typeName,
		Required: required,
	}
	// parse default value and convert to interface
	if defaultValue != nil {
		switch typeName {
		case "ID":
			fallthrough
		case "String":
			ts.Default = defaultValue.Raw
		case "Int":
			i, err := strconv.ParseInt(defaultValue.Raw, 10, 64)
			if err != nil {
				log.Warnf("Cannot parse input default for %s: %s", name, err)
			} else {
				ts.Default = i
			}
		case "Float":
			i, err := strconv.ParseFloat(defaultValue.Raw, 64)
			if err != nil {
				log.Warnf("Cannot parse input default for %s: %s", name, err)
			} else {
				ts.Default = i
			}
		case "Boolean":
			i, err := strconv.ParseBool(defaultValue.Raw)
			if err != nil {
				log.Warnf("Cannot parse input default for %s: %s", name, err)
			} else {
				ts.Default = i
			}
		default:
			log.Warnf("Unknown default value type: %s", typeName)
		}
	}
	return ts
}

func FlattenInput(parent string, input *ast.ArgumentDefinition, schema *ast.Schema) map[string]TypeSignature {
	ret := make(map[string]TypeSignature)

	def := schema.Types[input.Type.Name()]
	for _, field := range def.Fields {
		if IsScalar(field.Type.Name()) {
			flatName := fmt.Sprintf("%s.%s", parent, field.Name)
			ret[flatName] = MakeTypeSig(flatName, field.Type.Name(), field.Type.NonNull, nil)
		} else {
			log.Warnf("nested input types not supported at this time")
		}
	}
	return ret
}

func CreateGetMethod(name, parentPath, parentType string, parentFieldPath []FieldPathDetail, schema *ast.Schema) ([]*GetMethod, error) {
	return createGetMethodInner(name, parentPath, parentType, parentFieldPath, schema)
}

func createGetMethodInner(name, parentPath, parentType string, parentFieldPath []FieldPathDetail, schema *ast.Schema) ([]*GetMethod, error) {

	if len(parentFieldPath) > MAX_PATH_DEPTH {
		log.Warnf("createGetMethodInner: Max depth exceeded: %s/%s", parentPath, name)
		return nil, nil
	}

	var queryField *ast.FieldDefinition
	if parentType == "Query" {
		for _, field := range schema.Query.Fields {
			if field.Name == name {
				queryField = field
				break
			}
		}
	} else {
		parent := schema.Types[parentType]
		for _, field := range parent.Fields {
			if field.Name == name {
				queryField = field
				break
			}
		}
	}

	if parentFieldPath == nil {
		parentFieldPath = make([]FieldPathDetail, 0)
	} else {
		// naive detect cycle TODO - use type/param
		for _, item := range parentFieldPath {
			if item.Path == name {
				log.Debugf("Detected loop (%s), returning...", item)
				return nil, nil
			}
		}
	}

	if queryField == nil {
		return nil, fmt.Errorf("could not find query %s in schema", name)
	}

	newPath := fmt.Sprintf("%s/%s", parentPath, ToSnakeCase(queryField.Name))

	var sig *GetMethod

	if (parentType == "Query") || len(queryField.Arguments) > 0 {
		sig = &GetMethod{
			OriginalField: name,
			Path:          newPath,
			Method:        "GET",
			QueryString:   make(map[string]TypeSignature, len(queryField.Arguments)),
			FieldPath:     parentFieldPath,
		}
	}

	idInPath := false

	// handle input arguments
	for _, input := range queryField.Arguments {
		log.Infof("Type: name: %s, named type: %s", input.Name, input.Type.Name())

		// Try to encode ID into path to be more RESTy
		if (input.Name == "id") && (input.Type.Name() == "ID") {
			idInPath = true
			sig.IDInPath = true
			sig.Path = fmt.Sprintf("%s/:id", newPath)
			newPath = sig.Path
		} else {
			// If the input is a scalar, map it into the QS args.
			if IsScalar(input.Type.Name()) {
				sig.QueryString[input.Name] = MakeTypeSig(input.Name, input.Type.Name(), input.Type.NonNull, input.DefaultValue)
			} else {
				// otherwise flatten the input using dot notation
				log.Infof("Non scalar input, flattening...")
				typeMap := FlattenInput(input.Name, input, schema)
				maps.Copy(sig.QueryString, typeMap)
			}

		}
	}

	sigs := make([]*GetMethod, 0, 10)
	if sig != nil {
		sigs = append(sigs, sig)
	}

	if !IsScalar(queryField.Type.Name()) {
		// Here we need to decend into the return type to look for fields that take arguments
		// each of those will become it's own REST route.
		def := schema.Types[queryField.Type.Name()]

		for _, field := range def.Fields {
			// Fields with arguments must be represented by a different REST op
			// without some acrobatics in the way we represent query string parameters,
			// would need some kind of prefixing.
			if (len(field.Arguments) > 0) && ((len(queryField.Arguments) == 0) || (idInPath && (len(queryField.Arguments) == 1))) {
				innerSigs, _ := createGetMethodInner(
					field.Name,
					newPath,
					queryField.Type.Name(),
					append(parentFieldPath, FieldPathDetail{
						Path:     queryField.Name,
						IDInPath: idInPath,
					}),
					schema)

				if innerSigs != nil {
					sigs = append(sigs, innerSigs...)
				}

			} else {
				if IsScalar(field.Type.Name()) {
					if sig != nil {
						if sig.ResultSelections == nil {
							sig.ResultSelections = make([]string, 0, len(def.Fields))
						}
						sig.ResultSelections = append(sig.ResultSelections, field.Name)
					}

				} else {
					// This case is no arguments to the field and it's non-scalar
					// so we should search up through the tree to find terminal
					// nodes that will become their own REST routes.
					innerSigs, _ := createGetMethodInner(
						field.Name,
						newPath,
						queryField.Type.Name(),
						append(parentFieldPath, FieldPathDetail{
							Path:     queryField.Name,
							IDInPath: idInPath,
						}),
						schema)

					if innerSigs != nil {
						sigs = append(sigs, innerSigs...)
					}

				}
			}
		}

	} else {
		/// In this case the field returns a scalar type and has no selection set.
	}

	return sigs, nil
}

// Search type for inner fields
/*
func searchType(name, parentPath, parentType string, parentFieldPath []FieldPathDetail, schema *ast.Schema) ([]*GetMethod, error) {

	if len(parentFieldPath) > MAX_PATH_DEPTH {
		log.Warnf("searchType: Max depth exceeded: %s/%s", parentPath, name)
		return nil, nil
	}
	sigs := make([]*GetMethod, 0, 10)
	// naive detect cycle TODO - use type/param
	for _, item := range parentFieldPath {
		if item.Path == name {
			log.Debugf("Detected loop (%s), returning...", item)
			return sigs, nil
		}
	}

	parent := schema.Types[parentType]

	var queryField *ast.FieldDefinition
	for _, field := range parent.Fields {
		if field.Name == name {
			queryField = field
			break
		}
	}

	if queryField == nil {
		log.Warnf("Cannot found field %s on %s.", name, parentType)
		return nil, nil
	}

	log.Debugf("Searching type: %s %s %s %s", name, parentPath, parentType, parentFieldPath)

	newPath := fmt.Sprintf("%s/%s", parentPath, ToSnakeCase(queryField.Name))

	if !IsScalar(queryField.Type.Name()) {
		// Here we need to decend into the return type to look for fields that take arguments
		// each of those will become it's own REST route.
		def := schema.Types[queryField.Type.Name()]

		for _, field := range def.Fields {
			// Fields with arguments must be represented by a different REST op
			// without extreme agrubatics.
			if len(field.Arguments) > 0 {
				innerSigs, _ := createGetMethodInner(
					field.Name,
					newPath,
					queryField.Type.Name(),
					append(parentFieldPath, FieldPathDetail{
						Path:     queryField.Name,
						IDInPath: false,
					}),
					schema)

				if innerSigs != nil {
					sigs = append(sigs, innerSigs...)
				}

			} else {
				// We don't make a new REST route without args

				innerSigs, _ := searchType(
					field.Name,
					newPath,
					queryField.Type.Name(),
					append(parentFieldPath, FieldPathDetail{
						Path:     queryField.Name,
						IDInPath: false,
					}),
					schema)
				if innerSigs != nil {
					sigs = append(sigs, innerSigs...)
				}

			}
		}

	} else {
		log.Debugf("Field is scalar, returning.")
		/// In this case the field returns a scalar type and has no selection set.
	}
	return sigs, nil
}
*/
