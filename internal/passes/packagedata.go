package passes

import (
	"gocomp/internal/parser"
	"strings"
)

type PackageData struct {
	PackageName string
	Imports     []ImportAlias
	//Constants   []ConstantDecl
	Functions map[string]FunctionDecl
	Methods   map[string]map[string]FunctionDecl // receiver type -> method name -> decl
}
type ImportAlias struct {
	Path  string
	Alias string
}

//	type ConstantDecl struct {
//		Name  string
//		Value string
//	}

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

type PackageVisitor struct {
	parser.BaseGoParserVisitor
	pdata *PackageData
}

func NewPackageVisitor() *PackageVisitor {
	return &PackageVisitor{
		pdata: &PackageData{
			Functions: make(map[string]FunctionDecl),
			Methods:   make(map[string]map[string]FunctionDecl),
		},
	}
}

func (v *PackageVisitor) VisitSourceFile(ctx *parser.SourceFileContext) interface{} {
	v.VisitPackageClause(ctx.PackageClause().(*parser.PackageClauseContext))
	for _, child := range ctx.AllImportDecl() {
		v.VisitImportDecl(child.(*parser.ImportDeclContext))
	}
	for _, child := range ctx.AllFunctionDecl() {
		funDec := v.VisitFunctionDecl(child.(*parser.FunctionDeclContext)).(FunctionDecl)
		if _, ok := v.pdata.Functions[funDec.Name]; ok {
			panic("multiple functions with same name!")
		}
		v.pdata.Functions[funDec.Name] = funDec
	}
	for _, child := range ctx.AllMethodDecl() {
		funDec := v.VisitMethodDecl(child.(*parser.MethodDeclContext)).(FunctionDecl)
		if _, ok := v.pdata.Functions[funDec.Name]; ok {
			// TODO: does not apply to methods
			panic("multiple methods with same name!")
		}
		if _, ok := v.pdata.Methods[funDec.Receiver.Type]; !ok {
			v.pdata.Methods[funDec.Receiver.Type] = make(map[string]FunctionDecl)
		}
		v.pdata.Methods[funDec.Receiver.Type][funDec.Name] = funDec
	}
	return v.pdata
}

func (v *PackageVisitor) VisitPackageClause(ctx *parser.PackageClauseContext) interface{} {
	v.pdata.PackageName = ctx.GetPackageName().GetText()
	return nil
}

func (v *PackageVisitor) VisitImportDecl(ctx *parser.ImportDeclContext) interface{} {
	for _, spec := range ctx.AllImportSpec() {
		v.VisitImportSpec(spec.(*parser.ImportSpecContext))
	}
	return nil
}

func (v *PackageVisitor) VisitImportSpec(ctx *parser.ImportSpecContext) interface{} {
	path := strings.Join(strings.Split(ctx.ImportPath().GetText(), "\""), "")
	alias := path
	if ctx.GetAlias() != nil {
		alias = ctx.GetAlias().GetText()
	}
	v.pdata.Imports = append(v.pdata.Imports, ImportAlias{
		Path:  path,
		Alias: alias,
	})
	return v.VisitChildren(ctx)
}

func (v *PackageVisitor) VisitFunctionDecl(ctx *parser.FunctionDeclContext) interface{} {
	fundec := v.VisitSignature(ctx.Signature().(*parser.SignatureContext)).(FunctionDecl)
	fundec.Name = ctx.IDENTIFIER().GetText()
	return fundec
}

func (v *PackageVisitor) VisitMethodDecl(ctx *parser.MethodDeclContext) interface{} {
	fundec := v.VisitSignature(ctx.Signature().(*parser.SignatureContext)).(FunctionDecl)
	fundec.Name = ctx.IDENTIFIER().GetText()
	// parse receiver
	rDecl := ctx.Receiver().Parameters().ParameterDecl(0)
	fundec.Receiver = &TypedName{
		Type: rDecl.Type_().GetText(),
	}
	if rDecl.IdentifierList() != nil {
		fundec.Receiver.Name = rDecl.IdentifierList().IDENTIFIER(0).GetText()
	}
	return fundec
}

func (v *PackageVisitor) VisitSignature(ctx *parser.SignatureContext) interface{} {
	fundec := FunctionDecl{
		ArgTypes: v.VisitParameters(ctx.Parameters().(*parser.ParametersContext)).([]TypedName),
	}
	if ctx.Result() != nil {
		fundec.ReturnTypes = append(fundec.ReturnTypes, TypedName{
			Type: ctx.Result().Type_().GetText(),
		})
	}
	return fundec
}

func (v *PackageVisitor) VisitParameters(ctx *parser.ParametersContext) interface{} {
	paramList := []TypedName{}
	for _, child := range ctx.AllParameterDecl() {
		paramList = append(paramList,
			v.VisitParameterDecl(child.(*parser.ParameterDeclContext)).([]TypedName)...)
	}
	return paramList
}

func (v PackageVisitor) VisitParameterDecl(ctx *parser.ParameterDeclContext) interface{} {
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
