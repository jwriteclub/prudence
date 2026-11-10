// Copyright 2026 Christopher O'Connell
// SPDX-License-Identifier: Apache-2.0
// Derived from tla+.jj, Copyright (c) 2003 Compaq Corporation, MIT license; see NOTICE.
package generated

func sanyRegion69518(r *recognizer, f *FrameRecursive) error {
	if e := r.state.bpa(r, r.state.UniqueStringOf("Recursive")); e != nil {
		return e
	}
	r.state.expecting = r.state.UniqueStringOf("RECURSIVE")
	return nil
}
func sanyRegion69627(r *recognizer, f *FrameRecursive) (bool, error) {
	{
		n, e := r.state.NewTokenNode(r.state.mn, f.T)
		if e != nil {
			return false, e
		}
		if e = r.state.addHeir(n); e != nil {
			return false, e
		}
	}
	r.state.expecting = r.state.UniqueStringOf("Identifier, operator or _")
	return false, nil
}
func sanyRegion69762(r *recognizer, f *FrameRecursive) (bool, error) {
	if e := r.state.addHeir(f.Tn); e != nil {
		return false, e
	}
	r.state.expecting = r.state.UniqueStringOf(",")
	return false, nil
}
func sanyRegion69853(r *recognizer, f *FrameRecursive) (bool, error) {
	{
		n, e := r.state.NewTokenNode(r.state.mn, f.T)
		if e != nil {
			return false, e
		}
		if e = r.state.addHeir(n); e != nil {
			return false, e
		}
	}
	r.state.expecting = r.state.UniqueStringOf("Identifier, operator or _")
	return false, nil
}
func sanyRegion69996(r *recognizer, f *FrameRecursive) (bool, error) {
	if e := r.state.addHeir(f.Tn); e != nil {
		return false, e
	}
	r.state.expecting = r.state.UniqueStringOf("`,' or `)'")
	return false, nil
}
func sanyRegion70084(r *recognizer, f *FrameRecursive) (bool, error) {
	{
		v, e := r.state.getLastHeirs(r.c.Check)
		if e != nil {
			return false, e
		}
		f.Sn = v
	}
	if e := r.state.epa(); e != nil {
		return false, e
	}
	n, e := r.state.NewNode(r.state.mn, N_Recursive, f.Sn)
	if e != nil {
		return false, e
	}
	f.Result = n
	return true, nil
}
func sanyRegion70216(r *recognizer, f *FrameConstantDeclarationItems) error {
	if e := r.state.bpa(r, r.state.UniqueStringOf("Constant declaration items")); e != nil {
		return e
	}
	r.state.expecting = r.state.UniqueStringOf("Identifier, _ or prefix op")
	return nil
}
func sanyRegion70376(r *recognizer, f *FrameConstantDeclarationItems) (bool, error) {
	if e := r.state.addHeir(f.Tn); e != nil {
		return false, e
	}
	f.Kind = int32(N_IdentDecl)
	r.state.expecting = r.state.UniqueStringOf("(, comma, or Module Body")
	return false, nil
}
func sanyRegion71338(r *recognizer, f *FrameConstantDeclarationItems) (bool, error) {
	{
		n, e := r.state.NewTokenNode(r.state.mn, f.T)
		if e != nil {
			return false, e
		}
		if e = r.state.addHeir(n); e != nil {
			return false, e
		}
	}
	r.state.expecting = r.state.UniqueStringOf("_")
	return false, nil
}
func sanyRegion71415(r *recognizer, f *FrameConstantDeclarationItems) (bool, error) {
	{
		n, e := r.state.NewTokenNode(r.state.mn, f.T)
		if e != nil {
			return false, e
		}
		if e = r.state.addHeir(n); e != nil {
			return false, e
		}
	}
	r.state.expecting = r.state.UniqueStringOf("comma or )")
	return false, nil
}
func sanyRegion71505(r *recognizer, f *FrameConstantDeclarationItems) (bool, error) {
	{
		n, e := r.state.NewTokenNode(r.state.mn, f.T)
		if e != nil {
			return false, e
		}
		if e = r.state.addHeir(n); e != nil {
			return false, e
		}
	}
	r.state.expecting = r.state.UniqueStringOf("_")
	return false, nil
}
func sanyRegion71586(r *recognizer, f *FrameConstantDeclarationItems) (bool, error) {
	{
		n, e := r.state.NewTokenNode(r.state.mn, f.T)
		if e != nil {
			return false, e
		}
		if e = r.state.addHeir(n); e != nil {
			return false, e
		}
	}
	r.state.expecting = r.state.UniqueStringOf("comma or )")
	return false, nil
}
func sanyRegion71681(r *recognizer, f *FrameConstantDeclarationItems) (bool, error) {
	{
		n, e := r.state.NewTokenNode(r.state.mn, f.T)
		if e != nil {
			return false, e
		}
		if e = r.state.addHeir(n); e != nil {
			return false, e
		}
	}
	return false, nil
}
func sanyRegion71824(r *recognizer, f *FrameConstantDeclarationItems) (bool, error) {
	r.state.expecting = r.state.UniqueStringOf("_")
	f.Kind = int32(N_PrefixDecl)
	if e := r.state.addHeir(f.Tn); e != nil {
		return false, e
	}
	return false, nil
}
func sanyRegion71900(r *recognizer, f *FrameConstantDeclarationItems) (bool, error) {
	{
		n, e := r.state.NewTokenNode(r.state.mn, f.T)
		if e != nil {
			return false, e
		}
		if e = r.state.addHeir(n); e != nil {
			return false, e
		}
	}
	return false, nil
}
func sanyRegion71957(r *recognizer, f *FrameConstantDeclarationItems) (bool, error) {
	r.state.expecting = r.state.UniqueStringOf("prefix or postfix operator")
	return false, nil
}
func sanyRegion72089(r *recognizer, f *FrameConstantDeclarationItems) (bool, error) {
	r.state.expecting = r.state.UniqueStringOf("_")
	f.Kind = int32(N_InfixDecl)
	{
		n, e := r.state.NewTokenNode(r.state.mn, f.T)
		if e != nil {
			return false, e
		}
		if e = r.state.addHeir(n); e != nil {
			return false, e
		}
	}
	if e := r.state.addHeir(f.Tn); e != nil {
		return false, e
	}
	return false, nil
}
func sanyRegion72223(r *recognizer, f *FrameConstantDeclarationItems) (bool, error) {
	{
		n, e := r.state.NewTokenNode(r.state.mn, f.T)
		if e != nil {
			return false, e
		}
		if e = r.state.addHeir(n); e != nil {
			return false, e
		}
	}
	return false, nil
}
func sanyRegion72292(r *recognizer, f *FrameConstantDeclarationItems) (bool, error) {
	f.Kind = int32(N_PostfixDecl)
	{
		n, e := r.state.NewTokenNode(r.state.mn, f.T)
		if e != nil {
			return false, e
		}
		if e = r.state.addHeir(n); e != nil {
			return false, e
		}
	}
	if e := r.state.addHeir(f.Tn); e != nil {
		return false, e
	}
	return false, nil
}
func sanyRegion72404(r *recognizer, f *FrameConstantDeclarationItems) (bool, error) {
	{
		v, e := r.state.getLastHeirs(r.c.Check)
		if e != nil {
			return false, e
		}
		f.Sn = v
	}
	if e := r.state.epa(); e != nil {
		return false, e
	}
	n, e := r.state.NewNode(r.state.mn, int(f.Kind), f.Sn)
	if e != nil {
		return false, e
	}
	f.Result = n
	return true, nil
}
func sanyRegion75777(r *recognizer, f *FrameIdentifierTuple) error {
	if e := r.state.bpa(r, r.state.UniqueStringOf("Identifier tuple")); e != nil {
		return e
	}
	return nil
}
func sanyRegion75877(r *recognizer, f *FrameIdentifierTuple) (bool, error) {
	{
		n, e := r.state.NewTokenNode(r.state.mn, f.T)
		if e != nil {
			return false, e
		}
		if e = r.state.addHeir(n); e != nil {
			return false, e
		}
	}
	r.state.expecting = r.state.UniqueStringOf("Identifier or >>")
	return false, nil
}
func sanyRegion75973(r *recognizer, f *FrameIdentifierTuple) (bool, error) {
	if e := r.state.addHeir(f.Tn); e != nil {
		return false, e
	}
	r.state.expecting = r.state.UniqueStringOf("COMMA or >>")
	return false, nil
}
func sanyRegion76043(r *recognizer, f *FrameIdentifierTuple) (bool, error) {
	{
		n, e := r.state.NewTokenNode(r.state.mn, f.T)
		if e != nil {
			return false, e
		}
		if e = r.state.addHeir(n); e != nil {
			return false, e
		}
	}
	r.state.expecting = r.state.UniqueStringOf("COMMA or >>")
	return false, nil
}
func sanyRegion76136(r *recognizer, f *FrameIdentifierTuple) (bool, error) {
	if e := r.state.addHeir(f.Tn); e != nil {
		return false, e
	}
	r.state.expecting = r.state.UniqueStringOf("COMMA or >>")
	return false, nil
}
func sanyRegion76204(r *recognizer, f *FrameIdentifierTuple) (bool, error) {
	{
		n, e := r.state.NewTokenNode(r.state.mn, f.T)
		if e != nil {
			return false, e
		}
		if e = r.state.addHeir(n); e != nil {
			return false, e
		}
	}
	return false, nil
}
func sanyRegion76248(r *recognizer, f *FrameIdentifierTuple) (bool, error) {
	sn, e := r.state.getLastHeirs(r.c.Check)
	if e != nil {
		return false, e
	}
	if e := r.state.epa(); e != nil {
		return false, e
	}
	n, e := r.state.NewNode(r.state.mn, N_IdentifierTuple, sn)
	if e != nil {
		return false, e
	}
	f.Result = n
	return true, nil
}
func sanyRegion76389(r *recognizer, f *FrameIdentLHS) error {
	if e := r.state.bpa(r, r.state.UniqueStringOf("Identifier LHS")); e != nil {
		return e
	}
	return nil
}
func sanyRegion76472(r *recognizer, f *FrameIdentLHS) (bool, error) {
	if e := r.state.addHeir(f.Tn); e != nil {
		return false, e
	}
	r.state.expecting = r.state.UniqueStringOf("( or ==")
	return false, nil
}
func sanyRegion76530(r *recognizer, f *FrameIdentLHS) (bool, error) {
	{
		n, e := r.state.NewTokenNode(r.state.mn, f.T)
		if e != nil {
			return false, e
		}
		if e = r.state.addHeir(n); e != nil {
			return false, e
		}
	}
	r.state.expecting = r.state.UniqueStringOf("Identifier Declaration, prefix op, _ or )")
	return false, nil
}
func sanyRegion76678(r *recognizer, f *FrameIdentLHS) (bool, error) {
	if e := r.state.addHeir(f.Tn); e != nil {
		return false, e
	}
	r.state.expecting = r.state.UniqueStringOf("COMMA or )")
	return false, nil
}
func sanyRegion76744(r *recognizer, f *FrameIdentLHS) (bool, error) {
	{
		n, e := r.state.NewTokenNode(r.state.mn, f.T)
		if e != nil {
			return false, e
		}
		if e = r.state.addHeir(n); e != nil {
			return false, e
		}
	}
	r.state.expecting = r.state.UniqueStringOf("Identifier Declaration, prefix op or _")
	return false, nil
}
func sanyRegion76891(r *recognizer, f *FrameIdentLHS) (bool, error) {
	if e := r.state.addHeir(f.Tn); e != nil {
		return false, e
	}
	r.state.expecting = r.state.UniqueStringOf("COMMA or )")
	return false, nil
}
func sanyRegion76961(r *recognizer, f *FrameIdentLHS) (bool, error) {
	{
		n, e := r.state.NewTokenNode(r.state.mn, f.T)
		if e != nil {
			return false, e
		}
		if e = r.state.addHeir(n); e != nil {
			return false, e
		}
	}
	return false, nil
}
func sanyRegion77015(r *recognizer, f *FrameIdentLHS) (bool, error) {
	sn, e := r.state.getLastHeirs(r.c.Check)
	if e != nil {
		return false, e
	}
	if e := r.state.epa(); e != nil {
		return false, e
	}
	n, e := r.state.NewNode(r.state.mn, N_IdentLHS, sn)
	if e != nil {
		return false, e
	}
	f.Result = n
	return true, nil
}
func sanyRegion78114(r *recognizer, f *FrameIdentDecl) error {
	if e := r.state.bpa(r, r.state.UniqueStringOf("Identifier Declation")); e != nil {
		return e
	}
	return nil
}
func sanyRegion78203(r *recognizer, f *FrameIdentDecl) (bool, error) {
	if e := r.state.addHeir(f.Tn); e != nil {
		return false, e
	}
	r.state.expecting = r.state.UniqueStringOf("( or ...")
	return false, nil
}
func sanyRegion78261(r *recognizer, f *FrameIdentDecl) (bool, error) {
	{
		n, e := r.state.NewTokenNode(r.state.mn, f.T)
		if e != nil {
			return false, e
		}
		if e = r.state.addHeir(n); e != nil {
			return false, e
		}
	}
	r.state.expecting = r.state.UniqueStringOf("_")
	return false, nil
}
func sanyRegion78334(r *recognizer, f *FrameIdentDecl) (bool, error) {
	{
		n, e := r.state.NewTokenNode(r.state.mn, f.T)
		if e != nil {
			return false, e
		}
		if e = r.state.addHeir(n); e != nil {
			return false, e
		}
	}
	r.state.expecting = r.state.UniqueStringOf("COMMA or )")
	return false, nil
}
func sanyRegion78427(r *recognizer, f *FrameIdentDecl) (bool, error) {
	{
		n, e := r.state.NewTokenNode(r.state.mn, f.T)
		if e != nil {
			return false, e
		}
		if e = r.state.addHeir(n); e != nil {
			return false, e
		}
	}
	r.state.expecting = r.state.UniqueStringOf("_")
	return false, nil
}
func sanyRegion78502(r *recognizer, f *FrameIdentDecl) (bool, error) {
	{
		n, e := r.state.NewTokenNode(r.state.mn, f.T)
		if e != nil {
			return false, e
		}
		if e = r.state.addHeir(n); e != nil {
			return false, e
		}
	}
	r.state.expecting = r.state.UniqueStringOf("COMMA or )")
	return false, nil
}
func sanyRegion78592(r *recognizer, f *FrameIdentDecl) (bool, error) {
	{
		n, e := r.state.NewTokenNode(r.state.mn, f.T)
		if e != nil {
			return false, e
		}
		if e = r.state.addHeir(n); e != nil {
			return false, e
		}
	}
	return false, nil
}
func sanyRegion78640(r *recognizer, f *FrameIdentDecl) (bool, error) {
	sn, e := r.state.getLastHeirs(r.c.Check)
	if e != nil {
		return false, e
	}
	if e := r.state.epa(); e != nil {
		return false, e
	}
	n, e := r.state.NewNode(r.state.mn, N_IdentDecl, sn)
	if e != nil {
		return false, e
	}
	f.Result = n
	return true, nil
}
func sanyRegion77148(r *recognizer, f *FramePrefixLHS) error {
	f.Sn = make([]*SyntaxTreeNode, 2)
	if e := r.state.bpa(r, r.state.UniqueStringOf("Prefix LHS")); e != nil {
		return e
	}
	return nil
}
func sanyRegion77280(r *recognizer, f *FramePrefixLHS) (bool, error) {
	{
		store := sanyNodeSlot(f.Sn, 0)
		n, e := r.state.NewTokenNode(r.state.mn, f.T)
		if e != nil {
			return false, e
		}
		if e = store(n); e != nil {
			return false, e
		}
	}
	r.state.expecting = r.state.UniqueStringOf("Identifier")
	return false, nil
}
func sanyRegion77365(r *recognizer, f *FramePrefixLHS) (bool, error) {
	if e := sanyNodeSlot(f.Sn, 1)(f.Tn); e != nil {
		return false, e
	}
	return false, nil
}
func sanyRegion77383(r *recognizer, f *FramePrefixLHS) (bool, error) {
	if e := r.state.epa(); e != nil {
		return false, e
	}
	n, e := r.state.NewNode(r.state.mn, N_PrefixLHS, f.Sn)
	if e != nil {
		return false, e
	}
	f.Result = n
	return true, nil
}
func sanyRegion77476(r *recognizer, f *FrameInfixLHS) error {
	f.Sn = make([]*SyntaxTreeNode, 3)
	if e := r.state.bpa(r, r.state.UniqueStringOf("Infix LHS")); e != nil {
		return e
	}
	return nil
}
func sanyRegion77601(r *recognizer, f *FrameInfixLHS) (bool, error) {
	if e := sanyNodeSlot(f.Sn, 0)(f.Tn); e != nil {
		return false, e
	}
	return false, nil
}
func sanyRegion77640(r *recognizer, f *FrameInfixLHS) (bool, error) {
	{
		store := sanyNodeSlot(f.Sn, 1)
		n, e := r.state.NewTokenNode(r.state.mn, f.T)
		if e != nil {
			return false, e
		}
		if e = store(n); e != nil {
			return false, e
		}
	}
	return false, nil
}
func sanyRegion77699(r *recognizer, f *FrameInfixLHS) (bool, error) {
	if e := sanyNodeSlot(f.Sn, 2)(f.Tn); e != nil {
		return false, e
	}
	return false, nil
}
func sanyRegion77717(r *recognizer, f *FrameInfixLHS) (bool, error) {
	if e := r.state.epa(); e != nil {
		return false, e
	}
	n, e := r.state.NewNode(r.state.mn, N_InfixLHS, f.Sn)
	if e != nil {
		return false, e
	}
	f.Result = n
	return true, nil
}
func sanyRegion77811(r *recognizer, f *FramePostfixLHS) error {
	f.Sn = make([]*SyntaxTreeNode, 2)
	if e := r.state.bpa(r, r.state.UniqueStringOf("Postfix LHS")); e != nil {
		return e
	}
	return nil
}
func sanyRegion77938(r *recognizer, f *FramePostfixLHS) (bool, error) {
	if e := sanyNodeSlot(f.Sn, 0)(f.Tn); e != nil {
		return false, e
	}
	return false, nil
}
func sanyRegion77979(r *recognizer, f *FramePostfixLHS) (bool, error) {
	{
		store := sanyNodeSlot(f.Sn, 1)
		n, e := r.state.NewTokenNode(r.state.mn, f.T)
		if e != nil {
			return false, e
		}
		if e = store(n); e != nil {
			return false, e
		}
	}
	return false, nil
}
func sanyRegion78020(r *recognizer, f *FramePostfixLHS) (bool, error) {
	if e := r.state.epa(); e != nil {
		return false, e
	}
	n, e := r.state.NewNode(r.state.mn, N_PostfixLHS, f.Sn)
	if e != nil {
		return false, e
	}
	f.Result = n
	return true, nil
}
func sanyRegion73154(r *recognizer, f *FrameOperatorOrFunctionDefinition) error {
	f.Zn = nil
	if e := r.state.bpa(r, r.state.UniqueStringOf("Definition")); e != nil {
		return e
	}
	f.Kind = 0
	return nil
}
func sanyRegion73269(r *recognizer, f *FrameOperatorOrFunctionDefinition) (bool, error) {
	r.state.expecting = r.state.UniqueStringOf("LOCAL, Identifier or Operator Symbol")
	return false, nil
}
func sanyRegion73341(r *recognizer, f *FrameOperatorOrFunctionDefinition) (bool, error) {
	{
		n, e := r.state.NewTokenNode(r.state.mn, f.T)
		if e != nil {
			return false, e
		}
		f.Zn = n
	}
	return false, nil
}
func sanyRegion73409(r *recognizer, f *FrameOperatorOrFunctionDefinition) (bool, error) {
	r.state.expecting = r.state.UniqueStringOf("LOCAL, Identifier or Operator Symbol")
	return false, nil
}
func sanyRegion73552(r *recognizer, f *FrameOperatorOrFunctionDefinition) (bool, error) {
	if e := r.state.addHeir(f.Tn); e != nil {
		return false, e
	}
	f.Kind = int32(N_FunctionDefinition)
	r.state.expecting = r.state.UniqueStringOf("[")
	return false, nil
}
func sanyRegion73636(r *recognizer, f *FrameOperatorOrFunctionDefinition) (bool, error) {
	{
		n, e := r.state.NewTokenNode(r.state.mn, f.T)
		if e != nil {
			return false, e
		}
		if e = r.state.addHeir(n); e != nil {
			return false, e
		}
	}
	r.state.expecting = r.state.UniqueStringOf("Identifier")
	return false, nil
}
func sanyRegion73748(r *recognizer, f *FrameOperatorOrFunctionDefinition) (bool, error) {
	if e := r.state.addHeir(f.Tn); e != nil {
		return false, e
	}
	r.state.expecting = r.state.UniqueStringOf("COMMA or ]")
	return false, nil
}
func sanyRegion73815(r *recognizer, f *FrameOperatorOrFunctionDefinition) (bool, error) {
	{
		n, e := r.state.NewTokenNode(r.state.mn, f.T)
		if e != nil {
			return false, e
		}
		if e = r.state.addHeir(n); e != nil {
			return false, e
		}
	}
	r.state.expecting = r.state.UniqueStringOf("Identifier")
	return false, nil
}
func sanyRegion73911(r *recognizer, f *FrameOperatorOrFunctionDefinition) (bool, error) {
	if e := r.state.addHeir(f.Tn); e != nil {
		return false, e
	}
	return false, nil
}
func sanyRegion73980(r *recognizer, f *FrameOperatorOrFunctionDefinition) (bool, error) {
	r.state.expecting = r.state.UniqueStringOf("==")
	{
		n, e := r.state.NewTokenNode(r.state.mn, f.T)
		if e != nil {
			return false, e
		}
		if e = r.state.addHeir(n); e != nil {
			return false, e
		}
	}
	return false, nil
}
func sanyRegion74069(r *recognizer, f *FrameOperatorOrFunctionDefinition) (bool, error) {
	if e := BelchDEF(r, r.c.Check); e != nil {
		return false, e
	}
	{
		n, e := r.state.NewTokenNode(r.state.mn, f.T)
		if e != nil {
			return false, e
		}
		if e = r.state.addHeir(n); e != nil {
			return false, e
		}
	}
	r.state.expecting = r.state.UniqueStringOf("Expression")
	return false, nil
}
func sanyRegion74175(r *recognizer, f *FrameOperatorOrFunctionDefinition) (bool, error) {
	if e := r.state.addHeir(f.Tn); e != nil {
		return false, e
	}
	return false, nil
}
func sanyRegion74354(r *recognizer, f *FrameOperatorOrFunctionDefinition) (bool, error) {
	if e := r.state.addHeir(f.Tn); e != nil {
		return false, e
	}
	r.state.expecting = r.state.UniqueStringOf("==")
	return false, nil
}
func sanyRegion74411(r *recognizer, f *FrameOperatorOrFunctionDefinition) (bool, error) {
	if e := BelchDEF(r, r.c.Check); e != nil {
		return false, e
	}
	f.Kind = int32(N_OperatorDefinition)
	{
		n, e := r.state.NewTokenNode(r.state.mn, f.T)
		if e != nil {
			return false, e
		}
		if e = r.state.addHeir(n); e != nil {
			return false, e
		}
	}
	r.state.expecting = r.state.UniqueStringOf("Expression")
	return false, nil
}
func sanyRegion74557(r *recognizer, f *FrameOperatorOrFunctionDefinition) (bool, error) {
	if e := r.state.addHeir(f.Tn); e != nil {
		return false, e
	}
	return false, nil
}
func sanyRegion74730(r *recognizer, f *FrameOperatorOrFunctionDefinition) (bool, error) {
	if e := r.state.addHeir(f.Tn); e != nil {
		return false, e
	}
	r.state.expecting = r.state.UniqueStringOf("==")
	return false, nil
}
func sanyRegion74787(r *recognizer, f *FrameOperatorOrFunctionDefinition) (bool, error) {
	if e := BelchDEF(r, r.c.Check); e != nil {
		return false, e
	}
	f.Kind = int32(N_OperatorDefinition)
	{
		n, e := r.state.NewTokenNode(r.state.mn, f.T)
		if e != nil {
			return false, e
		}
		if e = r.state.addHeir(n); e != nil {
			return false, e
		}
	}
	r.state.expecting = r.state.UniqueStringOf("Expression")
	return false, nil
}
func sanyRegion74934(r *recognizer, f *FrameOperatorOrFunctionDefinition) (bool, error) {
	if e := r.state.addHeir(f.Tn); e != nil {
		return false, e
	}
	return false, nil
}
func sanyRegion75078(r *recognizer, f *FrameOperatorOrFunctionDefinition) (bool, error) {
	if e := r.state.addHeir(f.Tn); e != nil {
		return false, e
	}
	r.state.expecting = r.state.UniqueStringOf("==")
	return false, nil
}
func sanyRegion75134(r *recognizer, f *FrameOperatorOrFunctionDefinition) (bool, error) {
	if e := BelchDEF(r, r.c.Check); e != nil {
		return false, e
	}
	{
		n, e := r.state.NewTokenNode(r.state.mn, f.T)
		if e != nil {
			return false, e
		}
		if e = r.state.addHeir(n); e != nil {
			return false, e
		}
	}
	r.state.expecting = r.state.UniqueStringOf("Expression or Instance")
	return false, nil
}
func sanyRegion75255(r *recognizer, f *FrameOperatorOrFunctionDefinition) (bool, error) {
	f.Kind = int32(N_OperatorDefinition)
	return false, nil
}
func sanyRegion75321(r *recognizer, f *FrameOperatorOrFunctionDefinition) (bool, error) {
	f.Kind = int32(N_ModuleDefinition)
	return false, nil
}
func sanyRegion75362(r *recognizer, f *FrameOperatorOrFunctionDefinition) (bool, error) {
	if e := r.state.addHeir(f.Tn); e != nil {
		return false, e
	}
	return false, nil
}
func sanyRegion75410(r *recognizer, f *FrameOperatorOrFunctionDefinition) (bool, error) {
	if e := r.state.addHeir(f.Tn); e != nil {
		return false, e
	}
	r.state.expecting = r.state.UniqueStringOf("==")
	return false, nil
}
func sanyRegion75464(r *recognizer, f *FrameOperatorOrFunctionDefinition) (bool, error) {
	if e := BelchDEF(r, r.c.Check); e != nil {
		return false, e
	}
	f.Kind = int32(N_OperatorDefinition)
	{
		n, e := r.state.NewTokenNode(r.state.mn, f.T)
		if e != nil {
			return false, e
		}
		if e = r.state.addHeir(n); e != nil {
			return false, e
		}
	}
	r.state.expecting = r.state.UniqueStringOf("Expression")
	return false, nil
}
func sanyRegion75614(r *recognizer, f *FrameOperatorOrFunctionDefinition) (bool, error) {
	if e := r.state.addHeir(f.Tn); e != nil {
		return false, e
	}
	return false, nil
}
func sanyRegion75639(r *recognizer, f *FrameOperatorOrFunctionDefinition) (bool, error) {
	sn, e := r.state.getLastHeirs(r.c.Check)
	if e != nil {
		return false, e
	}
	if e := r.state.epa(); e != nil {
		return false, e
	}
	n, e := r.state.NewNodeOptional(r.state.mn, int(f.Kind), f.Zn, sn)
	if e != nil {
		return false, e
	}
	f.Result = n
	return true, nil
}
