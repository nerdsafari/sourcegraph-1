import * as drop from "lodash/drop";
import { FileLabel } from "vs/base/browser/ui/iconLabel/iconLabel";
import { IWorkspaceProvider } from "vs/base/common/labels";
import URI from "vs/base/common/uri";

// We override the file label because VSCode uses different URI conventions
// than we do. This is required to make the references view file list have
// reasonable names.
FileLabel.prototype.setFile = function (file: URI, provider: IWorkspaceProvider): void {
	const path = file.path + "/" + file.fragment;
	const dirs = drop(path.split("/"));
	const base = dirs.pop();
	this.setValue(base, dirs.join("/"));
};
