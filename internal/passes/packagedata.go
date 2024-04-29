package passes

import (
	"gocomp/internal/parser"
	"gocomp/internal/typesystem"
	"gocomp/internal/utils"
	"strings"

	"github.com/llir/llvm/ir/types"
)

type PackageData struct {
	PackageName string
	Imports     []ImportAlias

	Functions map[string]*FunctionDecl
	Methods   map[string]map[string]*FunctionDecl // receiver type -> method name -> decl

	*typeManager
}
type ImportAlias struct {
	Path  string
	Alias string
}

func (pd *PackageData) LookupModule(module string) (*typesystem.GoModule, bool) {
	for _, imp := range pd.Imports {
		if imp.Alias == module || imp.Path == module {
			return &typesystem.GoModule{Name: imp.Path}, true
		}
	}
	return nil, false
}

type FunctionDecl struct {
	Name        string
	Receiver    types.Type
	ReturnNames []string
	ReturnTypes []types.Type
	ArgNames    []string
	ArgTypes    []types.Type
}

type PackageListener struct {
	parser.BaseGoParserListener
	pdata *PackageData
	err   error
}

var _ parser.GoParserListener = new(PackageListener)

func NewPackageListener() *PackageListener {
	return &PackageListener{
		BaseGoParserListener: parser.BaseGoParserListener{},
		pdata: &PackageData{
			Functions:   make(map[string]*FunctionDecl),
			Methods:     make(map[string]map[string]*FunctionDecl),
			typeManager: newTypeManager(),
		},
	}
}

func (v *PackageListener) PackageData() (*PackageData, error) {
	if v.err != nil {
		return nil, v.err
	}
	return v.pdata, nil
}

func (v *PackageListener) EnterPackageClause(ctx *parser.PackageClauseContext) {
	v.pdata.PackageName = ctx.GetPackageName().GetText()
}

func (v *PackageListener) EnterImportSpec(ctx *parser.ImportSpecContext) {
	path := strings.Join(strings.Split(ctx.ImportPath().GetText(), "\""), "")
	alias := path
	if ctx.GetAlias() != nil {
		alias = ctx.GetAlias().GetText()
	}
	v.pdata.Imports = append(v.pdata.Imports, ImportAlias{
		Path:  path,
		Alias: alias,
	})
}

func (v *PackageListener) EnterTypeDecl(ctx *parser.TypeDeclContext) {
	err := v.pdata.ParseTypeDecl(ctx)
	if err != nil {
		print("error is not nil!! on enter type decl")
	}
}

func (v *PackageListener) EnterFunctionDecl(ctx *parser.FunctionDeclContext) {
	fundec, err := v.ParseSignature(ctx.Signature().(*parser.SignatureContext))
	if err != nil {
		v.err = err
		return
	}
	fundec.Name = v.pdata.PackageName + "__" + ctx.IDENTIFIER().GetText()
	v.pdata.Functions[fundec.Name] = fundec
}

func (v *PackageListener) EnterMethodDecl(ctx *parser.MethodDeclContext) {
	var err error
	fundec, err := v.ParseSignature(ctx.Signature())
	if err != nil {
		v.err = err
		return
	}
	fundec.Name = ctx.IDENTIFIER().GetText()
	// parse receiver
	rDecl := ctx.Receiver().Parameters().ParameterDecl(0)
	fundec.Receiver, err = v.pdata.ParseType(rDecl.Type_())
	if err != nil {
		v.err = err
		return
	} else if rDecl.IdentifierList() != nil {
		// fundec.Receiver.Name = rDecl.IdentifierList().IDENTIFIER(0).GetText()
	}
	// if _, ok := v.pdata.Methods[fundec.Receiver.Type]; !ok {
	// 	v.pdata.Methods[fundec.Receiver.Type] = make(map[string]FunctionDecl)
	// }
	// v.pdata.Methods[fundec.Receiver.Type][fundec.Name] = fundec
}

func (v *PackageListener) ParseSignature(ctx parser.ISignatureContext) (*FunctionDecl, error) {
	names, types, err := v.ParseParameters(ctx.Parameters())
	if err != nil {
		return nil, utils.MakeErrorTrace(ctx, err, "failed to parse signature")
	}
	fundec := FunctionDecl{
		ArgNames: names,
		ArgTypes: types,
	}
	if ctx.Result() != nil {
		// single return value
		if ctx.Result().Type_() != nil {
			tp, err := v.pdata.ParseType(ctx.Result().Type_())
			if err != nil {
				return nil, utils.MakeErrorTrace(ctx, err, "failed to parse signature")
			}
			fundec.ReturnNames = append(fundec.ReturnNames, "")
			fundec.ReturnTypes = append(fundec.ReturnTypes, tp)
		} else {
			// multiple return values
			names, types, err := v.ParseParameters(ctx.Result().Parameters())
			if err != nil {
				return nil, utils.MakeErrorTrace(ctx, err, "failed to parse signature")
			}
			fundec.ReturnNames = append(fundec.ReturnNames, names...)
			fundec.ReturnTypes = append(fundec.ReturnTypes, types...)
		}
	}
	return &fundec, nil
}

func (v *PackageListener) ParseParameters(ctx parser.IParametersContext) ([]string, []types.Type, error) {
	var names []string
	var types []types.Type
	for _, child := range ctx.AllParameterDecl() {
		newNames, newTypes, err := v.ParseParameterDecl(child)
		if err != nil {
			return nil, nil, utils.MakeErrorTrace(ctx, err, "failed to parse parameters")
		}
		names = append(names, newNames...)
		types = append(types, newTypes...)
	}
	return names, types, nil
}

func (v *PackageListener) ParseParameterDecl(ctx parser.IParameterDeclContext) ([]string, []types.Type, error) {
	type_, err := v.pdata.ParseType(ctx.Type_())
	if err != nil {
		return nil, nil, err
	}
	var names []string
	var types []types.Type
	if ctx.IdentifierList() != nil {
		for _, ident := range ctx.IdentifierList().AllIDENTIFIER() {
			names = append(names, ident.GetText())
			types = append(types, type_)
		}
	} else {
		// unnamed parameter
		names = append(names, "")
		types = append(types, type_)
	}
	return names, types, nil
}
