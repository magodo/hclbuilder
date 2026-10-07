func (r Resource) update(data acceptance.TestData) string {
	return fmt.Sprintf(`
resource "example_resource" "test" {
  name                            = "acctestvmss-%[1]d"
  resource_group_name             = azurerm_resource_group.test.name
  location                        = azurerm_resource_group.test.location
  sku                             = "Standard_D2s_v3"
  instances                       = 1
  admin_username                  = "adminuser"
  admin_password                  = "P@ssword1234!"
  zones                           = ["3"]
  single_placement_group          = false
  capacity_reservation_group_id   = azurerm_capacity_reservation_group.test2.id
  platform_fault_domain_count     = 1

  source_image_reference {
    publisher = "Canonical"
    offer     = "0001-com-ubuntu-server-jammy"
    sku       = "22_04-lts"
    version   = "latest"
  }

  os_disk {
    storage_account_type = "Standard_LRS"
    caching              = "ReadWrite"
  }

  network_interface {
    name    = "example"
    primary = true
  }
}
`, data.RandomInteger)
}
