package passes

import (
	"gocomp/internal/parser"
	"strings"
)

type PackageData struct {
	PackageName string
	Imports     []ImportAlias

	Functions map[string]FunctionDecl
	Methods   map[string]map[string]FunctionDecl // receiver type -> method name -> decl
}
type ImportAlias struct {
	Path  string
	Alias string
}

type FunctionDecl struct {
	Name        string
	Receiver    *TypedName
	ReturnTypes []TypedName
	ArgTypes    []TypedName
}

type TypedName struct {
	Name      string
	Type      string
	Elipsised bool
}

type PackageListener struct {
	parser.BaseGoParserListener
	pdata *PackageData
}

func NewPackageListener() *PackageListener {
	return &PackageListener{
		BaseGoParserListener: parser.BaseGoParserListener{},
		pdata: &PackageData{
			Functions: make(map[string]FunctionDecl),
			Methods:   make(map[string]map[string]FunctionDecl),
		},
	}
}

func (v *PackageListener) PackageData() *PackageData {
	return v.pdata
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

func (v *PackageListener) EnterFunctionDecl(ctx *parser.FunctionDeclContext) {
	fundec := v.ParseSignature(ctx.Signature().(*parser.SignatureContext))
	fundec.Name = ctx.IDENTIFIER().GetText()
	v.pdata.Functions[fundec.Name] = fundec
}

func (v *PackageListener) EnterMethodDecl(ctx *parser.MethodDeclContext) {
	fundec := v.ParseSignature(ctx.Signature())
	fundec.Name = ctx.IDENTIFIER().GetText()
	// parse receiver
	rDecl := ctx.Receiver().Parameters().ParameterDecl(0)
	fundec.Receiver = &TypedName{
		Type: rDecl.Type_().GetText(),
	}
	if rDecl.IdentifierList() != nil {
		fundec.Receiver.Name = rDecl.IdentifierList().IDENTIFIER(0).GetText()
	}
	if _, ok := v.pdata.Methods[fundec.Receiver.Type]; !ok {
		v.pdata.Methods[fundec.Receiver.Type] = make(map[string]FunctionDecl)
	}
	v.pdata.Methods[fundec.Receiver.Type][fundec.Name] = fundec
}

func (v *PackageListener) ParseSignature(ctx parser.ISignatureContext) FunctionDecl {
	fundec := FunctionDecl{
		ArgTypes: v.ParseParameters(ctx.Parameters()),
	}
	if ctx.Result() != nil {
		fundec.ReturnTypes = append(fundec.ReturnTypes, TypedName{
			Type: ctx.Result().Type_().GetText(),
		})
	}
	return fundec
}

func (v *PackageListener) ParseParameters(ctx parser.IParametersContext) []TypedName {
	paramList := []TypedName{}
	for _, child := range ctx.AllParameterDecl() {
		paramList = append(paramList, v.ParseParameterDecl(child)...)
	}
	return paramList
}

func (v PackageListener) ParseParameterDecl(ctx parser.IParameterDeclContext) []TypedName {
	type_ := ctx.Type_().TypeName().GetText()
	params := []TypedName{}
	for _, ident := range ctx.IdentifierList().AllIDENTIFIER() {
		params = append(params, TypedName{
			Name: ident.GetText(),
			Type: type_,
		})
	}
	params[len(params)-1].Elipsised = ctx.ELLIPSIS() != nil
	return params
}
